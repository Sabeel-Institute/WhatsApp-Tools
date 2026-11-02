package coursegroups

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/api"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/people"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/runs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

func (w *Workflow) loadRun(id string) (*runState, error) {
	run, err := w.deps.Runs.Get(id)
	if errors.Is(err, runs.ErrNotFound) || (err == nil && run.Workflow != ID) {
		return nil, api.NotFound("We couldn't find that activity. It may have been deleted.")
	} else if err != nil {
		return nil, api.Internal(err)
	}
	d, err := loadDetail(run)
	if err != nil {
		return nil, api.Internal(err)
	}
	return &runState{w: w, run: run, detail: d}, nil
}

// handleExport downloads the results as a spreadsheet file. It starts with
// a byte-order mark so Excel reads names in any language correctly.
func (w *Workflow) handleExport(rw http.ResponseWriter, r *http.Request) error {
	s, err := w.loadRun(r.PathValue("id"))
	if err != nil {
		return err
	}
	name := safeFileName(s.detail.GroupName) + " results.csv"
	rw.Header().Set("Content-Type", "text/csv; charset=utf-8")
	rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s",
		asciiOnly(name), url.PathEscape(name)))
	rw.Header().Set("Cache-Control", "no-store")
	_, _ = rw.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(rw)
	_ = cw.Write([]string{"Name", "Phone", "Result", "Invite sent", "Row in original file"})
	for _, p := range s.detail.People {
		invite := ""
		switch p.InviteStatus {
		case "sent":
			invite = "Yes"
		case "failed":
			invite = "Failed"
		}
		result := outcomeText(p.Outcome)
		if p.Outcome == OutcomePending && s.run.Status != runs.StatusRunning {
			result = "Not added (the run ended early)"
		}
		_ = cw.Write([]string{csvSafe(p.Name), p.Display, result, invite, strconv.Itoa(p.Row)})
	}
	cw.Flush()
	return nil
}

// csvSafe stops a spreadsheet program from treating a name as a formula.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" {
		return "Group"
	}
	return s
}

func asciiOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r > 126 || r == '"' {
			return '_'
		}
		return r
	}, s)
}

// handleInvites starts a job that sends a personal invite to everyone who
// couldn't be added directly. It is opt-in: the UI explains that each
// person receives a message from the connected account.
func (w *Workflow) handleInvites(rw http.ResponseWriter, r *http.Request) error {
	var req struct {
		Message string `json:"message"`
	}
	if err := api.Decode(r, &req); err != nil {
		return err
	}
	s, err := w.loadRun(r.PathValue("id"))
	if err != nil {
		return err
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		return api.BadRequest("no_message", "Please write the message people will receive.")
	}
	if len(msg) > 4000 {
		return api.BadRequest("message_too_long", "The message is too long. Please keep it under 4,000 characters.")
	}
	if s.detail.InviteLink == "" {
		return api.BadRequest("no_link", "This group has no invite link yet, so invites can't be sent.")
	}
	if !strings.Contains(msg, s.detail.InviteLink) {
		msg += "\n\n" + s.detail.InviteLink
	}
	var targets []int
	for i, p := range s.detail.People {
		if needsInvite(p) && p.InviteStatus != "sent" {
			targets = append(targets, i)
		}
	}
	if len(targets) == 0 {
		return api.BadRequest("nobody", "Everyone who needed an invite has already been sent one.")
	}
	if st := w.deps.WA.Status().State; st != whatsapp.StateConnected && st != whatsapp.StateConnecting {
		return api.FromWhatsApp(whatsapp.ErrNotLinked)
	}
	s.detail.InviteMessage = msg
	if err := s.save(); err != nil {
		return api.Internal(err)
	}
	snap, err := w.deps.Jobs.Start(jobs.Spec{Workflow: ID, Kind: "invites", RunID: s.run.ID,
		Title: "Sending invites for “" + s.detail.GroupName + "”"},
		func(ctl *jobs.Control) error { return s.sendInvites(ctl, targets, msg) })
	if errors.Is(err, jobs.ErrBusy) {
		return api.Conflict("busy", "The app is busy with another task. Please wait for it to finish, then try again.")
	} else if err != nil {
		return api.Internal(err)
	}
	api.Write(rw, http.StatusOK, map[string]any{"job": snap})
	return nil
}

func needsInvite(p Person) bool {
	o := whatsapp.AddOutcome(p.Outcome)
	return o == whatsapp.AddPrivacy || o == whatsapp.AddRecentlyLeft
}

// sendInvites messages each person one at a time with long pauses.
func (s *runState) sendInvites(ctl *jobs.Control, targets []int, msg string) error {
	deps := s.w.deps
	ctl.Phase("Sending invites one at a time")
	ctl.Progress(0, len(targets))
	defer func() {
		if err := s.save(); err != nil {
			deps.Log.Error("saving invites failed", "err", err)
		}
	}()
	for n, i := range targets {
		if err := ctl.Checkpoint(); err != nil {
			return err
		}
		p := &s.detail.People[i]
		_, err := people.Call(ctl, deps.WA, deps.Pacing, func() (struct{}, error) {
			ctx, cancel := ctl.Context()
			defer cancel()
			return struct{}{}, deps.WA.SendInvite(ctx, whatsapp.Participant{Phone: p.Phone, UserID: p.UserID},
				whatsapp.Invite{GroupID: s.detail.GroupID, GroupName: s.detail.GroupName, Link: s.detail.InviteLink,
					Message: msg, Token: p.Invite})
		})
		var failure *jobs.Failure
		switch {
		case errors.Is(err, jobs.ErrStopped):
			return err
		case errors.As(err, &failure) && !whatsapp.Retryable(whatsapp.KindOf(failure.Err)) &&
			whatsapp.KindOf(failure.Err) != whatsapp.KindLoggedOut:
			// This person can't be messaged; carry on with the others.
			p.InviteStatus = "failed"
			ctl.Count("inviteFailed", 1)
			ctl.Add(jobs.Entry{Title: displayName(*p), Detail: "Couldn't send the invite", Outcome: "invite_failed"})
		case err != nil:
			return err
		default:
			p.InviteStatus = "sent"
			ctl.Count("inviteSent", 1)
			ctl.Add(jobs.Entry{Title: displayName(*p), Detail: "Invite sent", Outcome: "invite_sent"})
		}
		ctl.Progress(n+1, len(targets))
		if err := s.save(); err != nil {
			return err
		}
		if n+1 < len(targets) {
			if err := ctl.Sleep(whatsapp.Between(deps.Pacing.InvitePauseMin, deps.Pacing.InvitePauseMax)); err != nil {
				return err
			}
		}
	}
	return nil
}

func displayName(p Person) string {
	if p.Name != "" {
		return p.Name
	}
	return p.Display
}
