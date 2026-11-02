package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// sseHub fans events out to every open /api/events stream.
type sseHub struct {
	mu      sync.Mutex
	clients map[chan sseEvent]struct{}
}

type sseEvent struct {
	typ  string
	data any
}

func (h *sseHub) add() chan sseEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients == nil {
		h.clients = map[chan sseEvent]struct{}{}
	}
	ch := make(chan sseEvent, 64)
	h.clients[ch] = struct{}{}
	return ch
}

func (h *sseHub) remove(ch chan sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}

func (h *sseHub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *sseHub) send(typ string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- sseEvent{typ, data}:
		default: // a stalled tab catches up from the next snapshot
		}
	}
}

func (h *sseHub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		delete(h.clients, ch)
		close(ch)
	}
}

// pumpEvents forwards WhatsApp and job events to the browser.
func (s *Server) pumpEvents() {
	waEvents, stopWA := s.deps.WA.Subscribe()
	defer stopWA()
	jobEvents, stopJobs := s.deps.Jobs.Subscribe()
	defer stopJobs()
	for {
		select {
		case <-s.stopHub:
			return
		case e, ok := <-waEvents:
			if !ok {
				return
			}
			switch e.Type {
			case whatsapp.EventStatus:
				s.sse.send("status", e.Status)
			case whatsapp.EventLogin:
				s.sse.send("login", loginView(e.Login))
			}
		case snap, ok := <-jobEvents:
			if !ok {
				return
			}
			s.sse.send("job", snap)
		}
	}
}

// handleEvents streams server-sent events. The stream opens with the
// current state so a reloaded tab is immediately correct.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return nil
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	ch := s.sse.add()
	defer s.sse.remove(ch)

	st := s.deps.WA.Status()
	_ = writeEvent(w, "status", st)
	if job := s.activeJob(); job != nil {
		_ = writeEvent(w, "job", job)
	}
	flusher.Flush()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			s.touch()
			return nil
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if err := writeEvent(w, e.typ, e.data); err != nil {
				return nil
			}
			flusher.Flush()
		case <-ping.C:
			s.touch()
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}
