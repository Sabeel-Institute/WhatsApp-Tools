package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"rsc.io/qr"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/api"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/csvfile"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/people"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/phone"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/runs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/settings"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/workflows"
)

func (s *Server) routes() {
	s.handle("GET /api/app", s.handleApp)
	s.handle("POST /api/heartbeat", func(w http.ResponseWriter, r *http.Request) error {
		api.Write(w, http.StatusOK, map[string]bool{"ok": true})
		return nil
	})
	s.handle("POST /api/quit", s.handleQuit)
	s.handle("GET /api/events", s.handleEvents)

	s.handle("GET /api/wa/status", func(w http.ResponseWriter, r *http.Request) error {
		api.Write(w, http.StatusOK, s.deps.WA.Status())
		return nil
	})
	s.handle("POST /api/wa/login/qr", s.handleLoginQR)
	s.handle("POST /api/wa/login/phone", s.handleLoginPhone)
	s.handle("POST /api/wa/login/cancel", func(w http.ResponseWriter, r *http.Request) error {
		s.deps.WA.CancelLogin()
		api.Write(w, http.StatusOK, map[string]bool{"ok": true})
		return nil
	})
	s.handle("POST /api/wa/logout", s.handleLogout)
	s.handle("GET /api/wa/communities", s.handleCommunities)

	s.handle("GET /api/countries", func(w http.ResponseWriter, r *http.Request) error {
		api.Write(w, http.StatusOK, phone.Countries())
		return nil
	})
	s.handle("GET /api/settings", s.handleGetSettings)
	s.handle("PUT /api/settings/country", s.handleSetCountry)
	s.handle("PUT /api/settings/mapping", s.handleSetMapping)

	s.handle("POST /api/sheets/parse", s.handleParseSheet)
	s.handle("POST /api/people/review", s.handleReview)
	s.handle("POST /api/people/check", s.handleCheck)

	s.handle("GET /api/jobs/active", func(w http.ResponseWriter, r *http.Request) error {
		api.Write(w, http.StatusOK, map[string]any{"job": s.activeJob()})
		return nil
	})
	s.handle("GET /api/jobs/{id}", s.handleGetJob)
	s.handle("POST /api/jobs/{id}/{action}", s.handleJobAction)

	s.handle("GET /api/runs", s.handleListRuns)
	s.handle("GET /api/runs/{id}", s.handleGetRun)

	if s.cfg.Demo {
		s.handle("POST /api/demo/simulate", s.handleSimulate)
	}
}

// ---- app ----

func (s *Server) handleApp(w http.ResponseWriter, r *http.Request) error {
	infos := make([]workflows.Info, len(s.workflows))
	for i, wf := range s.workflows {
		infos[i] = wf.Info()
	}
	api.Write(w, http.StatusOK, map[string]any{
		"name":          s.cfg.AppName,
		"version":       s.cfg.Version,
		"demo":          s.cfg.Demo,
		"dataDir":       s.cfg.DataDir,
		"workflows":     infos,
		"maxGroupSize":  whatsapp.MaxGroupSize,
		"maxNameLength": whatsapp.MaxGroupNameLength,
		"maxDescLength": whatsapp.MaxDescriptionLength,
	})
	return nil
}

func (s *Server) handleQuit(w http.ResponseWriter, r *http.Request) error {
	if s.deps.Jobs.Busy() {
		return api.Conflict("job_running",
			"The app is still working. Wait until it finishes, or stop it first, then quit.")
	}
	api.Write(w, http.StatusOK, map[string]bool{"ok": true})
	if s.cfg.Quit != nil {
		go func() {
			time.Sleep(300 * time.Millisecond) // let the response reach the browser
			s.cfg.Quit()
		}()
	}
	return nil
}

// ---- linking ----

// loginEventView is a LoginEvent with the QR code drawn as SVG.
type loginEventView struct {
	*whatsapp.LoginEvent
	QR        string  `json:"qr,omitempty"` // hide the raw code
	QRSVG     string  `json:"qrSvg,omitempty"`
	ExpiresIn float64 `json:"expiresIn,omitempty"` // seconds
}

func loginView(e *whatsapp.LoginEvent) loginEventView {
	v := loginEventView{LoginEvent: e, ExpiresIn: e.ExpiresIn.Seconds()}
	if e.QR != "" {
		v.QRSVG = qrSVG(e.QR)
	}
	return v
}

// qrSVG draws text as a QR code with a quiet zone, as one SVG path.
func qrSVG(text string) string {
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		return ""
	}
	n := code.Size
	const quiet = 4
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges" role="img" aria-label="QR code to scan with WhatsApp">`, n+2*quiet, n+2*quiet)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="#fff"/><path fill="#1d1712" d="`)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if code.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}

func (s *Server) handleLoginQR(w http.ResponseWriter, r *http.Request) error {
	if err := s.deps.WA.StartQRLogin(context.Background()); err != nil {
		return api.FromWhatsApp(err)
	}
	api.Write(w, http.StatusOK, map[string]bool{"ok": true})
	return nil
}

func (s *Server) handleLoginPhone(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Phone   string `json:"phone"`
		Country string `json:"country"`
	}
	if err := api.Decode(r, &req); err != nil {
		return err
	}
	if !phone.ValidRegion(req.Country) {
		return api.BadRequest("bad_country", "Please choose a country.")
	}
	n := phone.Normalize(req.Phone, req.Country)
	if !n.OK() {
		return api.BadRequest("bad_phone", n.Message)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	code, err := s.deps.WA.StartPhoneLogin(ctx, n.E164)
	if err != nil {
		if whatsapp.KindOf(err) == whatsapp.KindInvalid {
			return api.BadRequest("bad_phone",
				"WhatsApp didn't accept that number. Check that it's the number of the phone with WhatsApp, including the country.")
		}
		return api.FromWhatsApp(err)
	}
	api.Write(w, http.StatusOK, map[string]string{"code": code, "phone": n.Display})
	return nil
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) error {
	if s.deps.Jobs.Busy() {
		return api.Conflict("job_running", "The app is still working. Wait until it finishes, or stop it first.")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	err := s.deps.WA.Logout(ctx)
	s.deps.Checker.Forget()
	if err != nil {
		// The link was still removed locally.
		api.Write(w, http.StatusOK, map[string]any{"ok": true, "offline": true})
		return nil
	}
	api.Write(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (s *Server) handleCommunities(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	list, err := s.deps.WA.Communities(ctx)
	if err != nil {
		return api.FromWhatsApp(err)
	}
	api.Write(w, http.StatusOK, list)
	return nil
}

// ---- settings ----

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) error {
	st := s.deps.Settings.Get()
	api.Write(w, http.StatusOK, map[string]any{"defaultCountry": st.DefaultCountry})
	return nil
}

func (s *Server) handleSetCountry(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Country string `json:"country"`
	}
	if err := api.Decode(r, &req); err != nil {
		return err
	}
	if !phone.ValidRegion(req.Country) {
		return api.BadRequest("bad_country", "Please choose a country from the list.")
	}
	if err := s.deps.Settings.Update(func(st *settings.Settings) { st.DefaultCountry = req.Country }); err != nil {
		return api.Internal(err)
	}
	api.Write(w, http.StatusOK, map[string]bool{"ok": true})
	return nil
}

func (s *Server) handleSetMapping(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Headers []string         `json:"headers"`
		Mapping settings.Mapping `json:"mapping"`
	}
	if err := api.Decode(r, &req); err != nil {
		return err
	}
	switch req.Mapping.NameMode {
	case "none", "full", "split":
	default:
		return api.BadRequest("bad_mapping", "Please choose how names are written.")
	}
	if req.Mapping.Phone == "" || len(req.Headers) == 0 {
		return api.BadRequest("bad_mapping", "Please choose the phone number column.")
	}
	sig := settings.HeaderSignature(req.Headers)
	if err := s.deps.Settings.Update(func(st *settings.Settings) { st.Mappings[sig] = req.Mapping }); err != nil {
		return api.Internal(err)
	}
	api.Write(w, http.StatusOK, map[string]bool{"ok": true})
	return nil
}

// ---- spreadsheets and people ----

func (s *Server) handleParseSheet(w http.ResponseWriter, r *http.Request) error {
	name := r.URL.Query().Get("name")
	data, err := io.ReadAll(io.LimitReader(r.Body, csvfile.MaxFileSize+1))
	if err != nil {
		return api.BadRequest("read_failed", "We couldn't read that file. Please try choosing it again.")
	}
	table, err := csvfile.Parse(name, data)
	if err != nil {
		var pe *csvfile.Error
		if errors.As(err, &pe) {
			return api.BadRequest("bad_file", pe.Message)
		}
		return api.Internal(err)
	}
	sugg := csvfile.Suggest(table)
	var remembered *settings.Mapping
	if table.HasHeader {
		if m, ok := s.deps.Settings.Get().Mappings[settings.HeaderSignature(table.Headers())]; ok {
			remembered = &m
		}
	}
	api.Write(w, http.StatusOK, map[string]any{"table": table, "suggestion": sugg, "remembered": remembered})
	return nil
}

// maxRows bounds a review request; a WhatsApp group holds 1,024 people, so
// this is generous.
const maxRows = 20000

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Rows    []people.Row `json:"rows"`
		Country string       `json:"country"`
	}
	if err := api.Decode(r, &req); err != nil {
		return err
	}
	if !phone.ValidRegion(req.Country) {
		return api.BadRequest("bad_country", "Please choose a country from the list.")
	}
	if len(req.Rows) > maxRows {
		return api.BadRequest("too_many", fmt.Sprintf("This list has more than %d people. Please split it into smaller files.", maxRows))
	}
	self := ""
	if acct := s.deps.WA.Status().Account; acct != nil {
		self = acct.Phone
	}
	api.Write(w, http.StatusOK, people.ReviewRows(req.Rows, req.Country, self, s.deps.Checker.Lookup))
	return nil
}

// handleCheck starts a background job that looks up unchecked numbers.
func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Phones []string `json:"phones"`
	}
	if err := api.Decode(r, &req); err != nil {
		return err
	}
	if len(req.Phones) > maxRows {
		return api.BadRequest("too_many", "Too many numbers to check at once.")
	}
	for _, p := range req.Phones {
		if !strings.HasPrefix(p, "+") {
			return api.BadRequest("bad_phone", "The app sent a number it couldn't read. Please reload the page.")
		}
	}
	todo := s.deps.Checker.Unchecked(req.Phones)
	if len(todo) == 0 {
		api.Write(w, http.StatusOK, map[string]any{"job": nil})
		return nil
	}
	if st := s.deps.WA.Status().State; st != whatsapp.StateConnected && st != whatsapp.StateConnecting {
		return api.FromWhatsApp(whatsapp.ErrNotLinked)
	}
	snap, err := s.deps.Jobs.Start(jobs.Spec{Workflow: "people", Kind: "check", Title: "Checking WhatsApp accounts"},
		func(ctl *jobs.Control) error {
			ctl.Phase("Checking which numbers have WhatsApp")
			ctl.Progress(0, len(todo))
			return s.deps.Checker.Check(ctl, todo, func(done int) { ctl.Progress(done, len(todo)) })
		})
	if errors.Is(err, jobs.ErrBusy) {
		return api.Conflict("busy", "The app is busy with another task. The check will be possible once it finishes.")
	} else if err != nil {
		return api.Internal(err)
	}
	api.Write(w, http.StatusOK, map[string]any{"job": snap})
	return nil
}

// ---- jobs ----

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) error {
	snap, ok := s.deps.Jobs.Get(r.PathValue("id"))
	if !ok {
		return api.NotFound("That task isn't running any more.")
	}
	api.Write(w, http.StatusOK, snap)
	return nil
}

func (s *Server) handleJobAction(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	var ok bool
	switch r.PathValue("action") {
	case "pause":
		ok = s.deps.Jobs.Pause(id)
	case "resume":
		ok = s.deps.Jobs.Resume(id)
	case "stop":
		ok = s.deps.Jobs.Stop(id)
	default:
		return api.NotFound("Unknown action.")
	}
	if !ok {
		return api.Conflict("job_state", "That isn't possible right now. The task may have just finished.")
	}
	snap, _ := s.deps.Jobs.Get(id)
	api.Write(w, http.StatusOK, snap)
	return nil
}

// ---- runs ----

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) error {
	list, err := s.deps.Runs.List(50)
	if err != nil {
		return api.Internal(err)
	}
	if list == nil {
		list = []runs.Run{}
	}
	api.Write(w, http.StatusOK, list)
	return nil
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) error {
	run, err := s.deps.Runs.Get(r.PathValue("id"))
	if errors.Is(err, runs.ErrNotFound) {
		return api.NotFound("We couldn't find that activity. It may have been deleted.")
	} else if err != nil {
		return api.Internal(err)
	}
	api.Write(w, http.StatusOK, run)
	return nil
}

// ---- demo ----

func (s *Server) handleSimulate(w http.ResponseWriter, r *http.Request) error {
	sim, ok := s.deps.WA.(interface{ Simulate(string) error })
	if !ok {
		return api.NotFound("Not available.")
	}
	var req struct {
		What string `json:"what"`
	}
	if err := api.Decode(r, &req); err != nil {
		return err
	}
	if err := sim.Simulate(req.What); err != nil {
		return api.BadRequest("bad_simulation", err.Error())
	}
	api.Write(w, http.StatusOK, map[string]bool{"ok": true})
	return nil
}
