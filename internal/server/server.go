// Package server is the local web server behind the UI: it serves the
// embedded web app, the JSON API and a stream of live events, and only ever
// answers the browser tab the app itself opened (see security.go).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/api"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/workflows"
)

// Config is fixed for the server's lifetime.
type Config struct {
	AppName    string
	Version    string
	Demo       bool
	Token      string
	DataDir    string
	DevOrigins []string // extra allowed origins in development
	Static     fs.FS    // the built web app
	Log        *slog.Logger
	// Quit is called when the user chooses Quit.
	Quit func()
}

// Server is the HTTP server.
type Server struct {
	cfg       Config
	deps      workflows.Deps
	workflows []workflows.Workflow
	mux       *http.ServeMux
	http      *http.Server
	port      atomic.Int32
	lastSeen  atomic.Int64
	sse       sseHub
	stopOnce  sync.Once
	stopHub   chan struct{}
}

// New builds the server and registers every route.
func New(cfg Config, deps workflows.Deps, wfs []workflows.Workflow) *Server {
	s := &Server{cfg: cfg, deps: deps, workflows: wfs, mux: http.NewServeMux(), stopHub: make(chan struct{})}
	s.touch()
	s.routes()
	for _, wf := range wfs {
		wf.Routes(prefixRouter{s: s, prefix: "/api/workflows/" + wf.Info().ID})
	}
	s.mux.Handle("/", s.static())
	s.http = &http.Server{
		Handler:           s.guard(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

type prefixRouter struct {
	s      *Server
	prefix string
}

func (p prefixRouter) Handle(pattern string, h api.HandlerFunc) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		method, path = "", pattern
	}
	full := p.prefix + path
	if method != "" {
		full = method + " " + full
	}
	p.s.mux.HandleFunc(full, api.Handle(p.s.cfg.Log, h))
}

func (s *Server) handle(pattern string, h api.HandlerFunc) {
	s.mux.HandleFunc(pattern, api.Handle(s.cfg.Log, h))
}

func (s *Server) portStr() string { return strconv.Itoa(int(s.port.Load())) }

// touch records that the browser is still around.
func (s *Server) touch() { s.lastSeen.Store(time.Now().UnixNano()) }

// IdleFor reports how long since the browser last made a request or the
// event stream was last open.
func (s *Server) IdleFor() time.Duration {
	if s.sse.count() > 0 {
		return 0
	}
	return time.Since(time.Unix(0, s.lastSeen.Load()))
}

// Listen binds 127.0.0.1:port (0 picks a free port).
func (s *Server) Listen(port int) (net.Listener, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	s.port.Store(int32(ln.Addr().(*net.TCPAddr).Port))
	return ln, nil
}

// Port returns the bound port.
func (s *Server) Port() int { return int(s.port.Load()) }

// Serve runs until Shutdown.
func (s *Server) Serve(ln net.Listener) error {
	go s.pumpEvents()
	err := s.http.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stopHub) })
	s.sse.closeAll()
	return s.http.Shutdown(ctx)
}

// static serves the embedded web app. Hashed assets are cached forever; the
// page itself never is, so an updated app is picked up immediately.
func (s *Server) static() http.Handler {
	files := http.FileServerFS(s.cfg.Static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || path == "index.html" {
			s.serveIndex(w)
			return
		}
		if _, err := fs.Stat(s.cfg.Static, path); err != nil {
			s.serveIndex(w)
			return
		}
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

// csp allows only this app's own files: the UI can make no request to the
// internet even if a dependency tried to.
const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; " +
	"font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

func (s *Server) serveIndex(w http.ResponseWriter) {
	data, err := fs.ReadFile(s.cfg.Static, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintln(w, "The user interface was not built into this copy of the app. Developers: run `make web` and rebuild.")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", csp)
	_, _ = w.Write(data)
}

// writeEvent formats one server-sent event.
func writeEvent(w http.ResponseWriter, typ string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ, data)
	return err
}

// jobSnapshot is a helper used by handlers.
func (s *Server) activeJob() *jobs.Snapshot {
	if snap, ok := s.deps.Jobs.Active(); ok {
		return &snap
	}
	return nil
}
