package coursegroups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/people"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/runs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// Counter keys, shared with the web UI.
const (
	countAdded         = "added"
	countAlready       = "already"
	countInvite        = "invite" // privacy settings or left recently
	countNotOnWhatsApp = "notOnWhatsApp"
	countFailed        = "failed"
	countPending       = "pending"
)

// outcomeText is the plain-language label for each outcome.
func outcomeText(o Outcome) string {
	switch whatsapp.AddOutcome(o) {
	case whatsapp.AddAdded:
		return "Added"
	case whatsapp.AddAlreadyMember:
		return "Already in the group"
	case whatsapp.AddPrivacy:
		return "Needs an invite: their privacy settings don't allow being added"
	case whatsapp.AddRecentlyLeft:
		return "Needs an invite: they left this group recently"
	case whatsapp.AddNotOnWhatsApp:
		return "Not on WhatsApp"
	case whatsapp.AddFailed:
		return "Couldn't be added"
	}
	return "Not added yet"
}

func counterFor(o Outcome) string {
	switch whatsapp.AddOutcome(o) {
	case whatsapp.AddAdded:
		return countAdded
	case whatsapp.AddAlreadyMember:
		return countAlready
	case whatsapp.AddPrivacy, whatsapp.AddRecentlyLeft:
		return countInvite
	case whatsapp.AddNotOnWhatsApp:
		return countNotOnWhatsApp
	case whatsapp.AddFailed:
		return countFailed
	}
	return countPending
}

// runState is one run in progress.
type runState struct {
	w      *Workflow
	run    runs.Run
	detail Detail
}

func (s *runState) counts() map[string]int {
	c := map[string]int{countAdded: 0, countAlready: 0, countInvite: 0, countNotOnWhatsApp: 0, countFailed: 0, countPending: 0}
	for _, p := range s.detail.People {
		c[counterFor(p.Outcome)]++
	}
	return c
}

func (s *runState) save() error {
	c := s.counts()
	s.run.Counts = c
	s.run.Headline = fmt.Sprintf("%d added", c[countAdded])
	if c[countInvite] > 0 {
		s.run.Headline += fmt.Sprintf(", %d need an invite", c[countInvite])
	}
	data, err := json.Marshal(s.detail)
	if err != nil {
		return err
	}
	s.run.Detail = data
	return s.w.deps.Runs.Save(s.run)
}

func (s *runState) entry(p Person) jobs.Entry {
	title, detail := p.Name, p.Display
	if title == "" {
		title, detail = p.Display, ""
	}
	return jobs.Entry{Title: title, Detail: detail, Outcome: string(p.Outcome)}
}

// execute is the job: check numbers, create the group, add people in paced
// batches, then fetch the invite link. Results are saved after every step,
// so a stop, failure or crash keeps everything done so far.
func (s *runState) execute(ctl *jobs.Control) (err error) {
	deps := s.w.deps
	total := len(s.detail.People)
	defer func() { s.finish(ctl, err) }()

	// 1. Every number must have been looked up; most were during review.
	phones := make([]string, total)
	for i, p := range s.detail.People {
		phones[i] = p.Phone
	}
	if todo := deps.Checker.Unchecked(phones); len(todo) > 0 {
		ctl.Phase("Checking which numbers have WhatsApp")
		ctl.Progress(0, len(todo))
		if err := deps.Checker.Check(ctl, todo, func(done int) { ctl.Progress(done, len(todo)) }); err != nil {
			return err
		}
	}
	done := 0
	for i := range s.detail.People {
		p := &s.detail.People[i]
		check, _ := deps.Checker.Get(p.Phone)
		if !check.OnWhatsApp {
			p.Outcome, p.Note = Outcome(whatsapp.AddNotOnWhatsApp), "This number doesn't have a WhatsApp account."
			ctl.Count(countNotOnWhatsApp, 1)
			done++
			continue
		}
		p.UserID = check.UserID
	}
	ctl.Progress(done, total)
	if err := s.save(); err != nil {
		return err
	}

	// 2. Create the group.
	if s.detail.Mode == "create" {
		if err := ctl.Checkpoint(); err != nil {
			return err
		}
		ctl.Phase("Creating the group “" + s.detail.GroupName + "”")
		group, err := people.Call(ctl, deps.WA, deps.Pacing, func() (whatsapp.Group, error) {
			ctx, cancel := ctl.Context()
			defer cancel()
			g, err := deps.WA.CreateGroup(ctx, whatsapp.CreateGroupRequest{
				Name: s.detail.GroupName, Description: s.detail.Description, CommunityID: s.detail.CommunityID,
				OnlyAdminsSend: s.detail.Settings.OnlyAdminsSend, OnlyAdminsEdit: s.detail.Settings.OnlyAdminsEdit,
				ApprovalRequired: s.detail.Settings.ApprovalRequired})
			var descErr *whatsapp.DescriptionError
			if errors.As(err, &descErr) {
				s.detail.Warnings = append(s.detail.Warnings,
					"The group was created, but WhatsApp didn't accept the description. You can add it in WhatsApp under Group info.")
				return g, nil
			}
			return g, err
		})
		if err != nil {
			return err
		}
		s.detail.GroupID = group.ID
		if err := s.save(); err != nil {
			return err
		}
		ctl.Add(jobs.Entry{Title: "Group created", Detail: s.detail.GroupName, Outcome: "created"})
	}

	// 3. Add people in small batches with randomized pauses.
	var pending []int
	for i, p := range s.detail.People {
		if p.Outcome == OutcomePending {
			pending = append(pending, i)
		}
	}
	size := max(deps.Pacing.AddBatchSize, 1)
	for start := 0; start < len(pending); start += size {
		if err := ctl.Checkpoint(); err != nil {
			return err
		}
		idx := pending[start:min(start+size, len(pending))]
		batch := make([]whatsapp.Participant, len(idx))
		for j, i := range idx {
			batch[j] = whatsapp.Participant{Phone: s.detail.People[i].Phone, UserID: s.detail.People[i].UserID}
		}
		ctl.Phase(fmt.Sprintf("Adding people to “%s”", s.detail.GroupName))
		results, err := people.Call(ctl, deps.WA, deps.Pacing, func() ([]whatsapp.AddResult, error) {
			ctx, cancel := ctl.Context()
			defer cancel()
			return deps.WA.AddParticipants(ctx, s.detail.GroupID, batch)
		})
		if err != nil {
			return err
		}
		var entries []jobs.Entry
		for j, res := range results {
			p := &s.detail.People[idx[j]]
			p.Outcome = Outcome(res.Outcome)
			p.Note = outcomeText(p.Outcome)
			p.Invite = res.Invite
			ctl.Count(counterFor(p.Outcome), 1)
			entries = append(entries, s.entry(*p))
			if res.Outcome == whatsapp.AddFailed {
				deps.Log.Info("participant not added", "code", res.Code)
			}
		}
		done += len(idx)
		ctl.Add(entries...)
		ctl.Progress(done, total)
		if err := s.save(); err != nil {
			return err
		}
		if start+size < len(pending) {
			ctl.Phase("Pausing a few seconds between batches to keep the account safe")
			if err := ctl.Sleep(whatsapp.Between(deps.Pacing.BatchPauseMin, deps.Pacing.BatchPauseMax)); err != nil {
				return err
			}
		}
	}

	// 4. The invite link, for everyone who couldn't be added directly.
	ctl.Phase("Getting the group's invite link")
	link, err := people.Call(ctl, deps.WA, deps.Pacing, func() (string, error) {
		ctx, cancel := ctl.Context()
		defer cancel()
		return deps.WA.InviteLink(ctx, s.detail.GroupID)
	})
	if errors.Is(err, jobs.ErrStopped) {
		return err
	} else if err != nil {
		s.detail.Warnings = append(s.detail.Warnings,
			"We couldn't get the group's invite link. In WhatsApp, open the group, tap its name, then “Invite via link”.")
	}
	s.setLink(link)
	return nil
}

// setLink records the invite link and the ready-to-send message.
func (s *runState) setLink(link string) {
	if link == "" {
		return
	}
	s.detail.InviteLink = link
	s.detail.InviteMessage = DefaultInviteMessage(s.detail.GroupName, link)
}

// DefaultInviteMessage is the friendly message for people who couldn't be
// added directly.
func DefaultInviteMessage(group, link string) string {
	return "Assalamu alaikum! You registered for " + group + ". We tried to add you to the course WhatsApp group, " +
		"but your privacy settings didn't allow it. Please join using this link: " + link
}

// finish records the final state, trying once for the invite link if the
// run ended early after the group was created.
func (s *runState) finish(ctl *jobs.Control, err error) {
	now := time.Now()
	s.run.FinishedAt = &now
	var failure *jobs.Failure
	switch {
	case err == nil:
		s.run.Status = runs.StatusCompleted
	case errors.Is(err, jobs.ErrStopped):
		s.run.Status = runs.StatusStopped
		s.run.Error = "You stopped this before everyone was added. Everything done up to that point is kept."
	case errors.As(err, &failure):
		s.run.Status = runs.StatusFailed
		s.run.Error = failure.Message
	default:
		s.run.Status = runs.StatusFailed
		s.run.Error = "Something unexpected went wrong. The results so far are saved."
		s.w.deps.Log.Error("run failed", "err", err)
	}
	if err != nil && s.detail.GroupID != "" && s.detail.InviteLink == "" &&
		s.w.deps.WA.Status().State == whatsapp.StateConnected {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		link, lerr := s.w.deps.WA.InviteLink(ctx, s.detail.GroupID)
		cancel()
		if lerr == nil {
			s.setLink(link)
		}
	}
	if saveErr := s.save(); saveErr != nil {
		s.w.deps.Log.Error("saving run failed", "err", saveErr)
	}
}
