// Package coursegroups is the "Course groups" workflow: create a group inside
// a WhatsApp Community from a registration spreadsheet (or add late
// registrants to an existing group), in small paced batches, then report who
// was added and help invite everyone who couldn't be.
package coursegroups

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/api"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/runs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/workflows"
)

// ID is the workflow's identifier in URLs and run history.
const ID = "course-groups"

// Workflow implements workflows.Workflow.
type Workflow struct {
	deps workflows.Deps
}

// New returns the workflow.
func New(deps workflows.Deps) *Workflow { return &Workflow{deps: deps} }

// Info implements workflows.Workflow.
func (w *Workflow) Info() workflows.Info {
	return workflows.Info{
		ID:          ID,
		Title:       "Course groups",
		Description: "Create a WhatsApp group for a course and add everyone who registered.",
		Actions: []workflows.Action{
			{ID: "create", Label: "Create a course group", Primary: true, Icon: "users-round",
				Description: "Start a new group in your Community and add everyone from a registration list."},
			{ID: "add", Label: "Add people to an existing group", Icon: "user-plus",
				Description: "For late registrants: add people from a list to a group you already have."},
		},
	}
}

// Routes implements workflows.Workflow.
func (w *Workflow) Routes(r api.Router) {
	r.Handle("POST /run", w.handleRun)
	r.Handle("GET /runs/{id}/export", w.handleExport)
	r.Handle("POST /runs/{id}/invites", w.handleInvites)
}

// Outcome of one person in a run. It extends whatsapp.AddOutcome with
// "pending" for people not reached yet.
type Outcome string

const (
	OutcomePending = Outcome("pending")
)

// Person is one row of a run's results.
type Person struct {
	Row          int                   `json:"row"`
	Name         string                `json:"name"`
	Phone        string                `json:"phone"`
	Display      string                `json:"display"`
	Outcome      Outcome               `json:"outcome"`
	Note         string                `json:"note,omitempty"`
	UserID       string                `json:"userId,omitempty"`
	Invite       *whatsapp.InviteToken `json:"invite,omitempty"`
	InviteStatus string                `json:"inviteStatus,omitempty"` // "", "sent", "failed"
}

// Settings are the optional group settings.
type Settings struct {
	OnlyAdminsSend   bool `json:"onlyAdminsSend"`
	OnlyAdminsEdit   bool `json:"onlyAdminsEdit"`
	ApprovalRequired bool `json:"approvalRequired"`
}

// Detail is the workflow-specific part of a run.
type Detail struct {
	Mode          string   `json:"mode"` // "create" or "add"
	CommunityID   string   `json:"communityId"`
	CommunityName string   `json:"communityName"`
	GroupID       string   `json:"groupId,omitempty"`
	GroupName     string   `json:"groupName"`
	Description   string   `json:"description,omitempty"`
	Settings      Settings `json:"settings"`
	InviteLink    string   `json:"inviteLink,omitempty"`
	InviteMessage string   `json:"inviteMessage,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	People        []Person `json:"people"`
}

// RunRequest starts a run.
type RunRequest struct {
	Mode          string   `json:"mode"`
	CommunityID   string   `json:"communityId"`
	CommunityName string   `json:"communityName"`
	GroupID       string   `json:"groupId"`
	GroupName     string   `json:"groupName"`
	Description   string   `json:"description"`
	Settings      Settings `json:"settings"`
	People        []struct {
		Row     int    `json:"row"`
		Name    string `json:"name"`
		Phone   string `json:"phone"`
		Display string `json:"display"`
	} `json:"people"`
}

func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }

func (req *RunRequest) validate() *api.Error {
	req.GroupName = strings.TrimSpace(req.GroupName)
	req.Description = strings.TrimSpace(req.Description)
	switch req.Mode {
	case "create":
		if req.CommunityID == "" {
			return api.BadRequest("no_community", "Please choose the Community for the new group.")
		}
		if req.GroupName == "" {
			return api.BadRequest("no_name", "Please give the group a name.")
		}
		if utf16Len(req.GroupName) > whatsapp.MaxGroupNameLength {
			return api.BadRequest("name_too_long", fmt.Sprintf("WhatsApp group names can be at most %d characters. Please shorten the name.", whatsapp.MaxGroupNameLength))
		}
		if utf16Len(req.Description) > whatsapp.MaxDescriptionLength {
			return api.BadRequest("description_too_long", fmt.Sprintf("The description can be at most %d characters.", whatsapp.MaxDescriptionLength))
		}
		if len(req.People)+1 > whatsapp.MaxGroupSize {
			return api.BadRequest("too_many", fmt.Sprintf("A WhatsApp group can have at most %d members, including you. Please split the list.", whatsapp.MaxGroupSize))
		}
	case "add":
		if req.GroupID == "" {
			return api.BadRequest("no_group", "Please choose the group to add people to.")
		}
		if len(req.People) >= whatsapp.MaxGroupSize {
			return api.BadRequest("too_many", fmt.Sprintf("A WhatsApp group can have at most %d members. Please split the list.", whatsapp.MaxGroupSize))
		}
	default:
		return api.BadRequest("bad_mode", "The app sent a request it couldn't read. Please reload the page.")
	}
	if len(req.People) == 0 {
		return api.BadRequest("no_people", "There's nobody to add. Go back and include at least one person.")
	}
	seen := map[string]bool{}
	for _, p := range req.People {
		if !strings.HasPrefix(p.Phone, "+") || len(p.Phone) < 8 || len(p.Phone) > 16 {
			return api.BadRequest("bad_phone", "The app sent a number it couldn't read. Please go back to the review step.")
		}
		if seen[p.Phone] {
			return api.BadRequest("duplicate", "The list contains the same number twice. Please go back to the review step.")
		}
		seen[p.Phone] = true
	}
	return nil
}

func (w *Workflow) handleRun(rw http.ResponseWriter, r *http.Request) error {
	var req RunRequest
	if err := api.Decode(r, &req); err != nil {
		return err
	}
	if err := req.validate(); err != nil {
		return err
	}
	if st := w.deps.WA.Status().State; st != whatsapp.StateConnected && st != whatsapp.StateConnecting {
		return api.FromWhatsApp(whatsapp.ErrNotLinked)
	}

	detail := Detail{Mode: req.Mode, CommunityID: req.CommunityID, CommunityName: req.CommunityName,
		GroupID: req.GroupID, GroupName: req.GroupName, Description: req.Description, Settings: req.Settings}
	if req.Mode == "add" {
		detail.Description, detail.Settings = "", Settings{}
	}
	for _, p := range req.People {
		detail.People = append(detail.People, Person{Row: p.Row, Name: strings.TrimSpace(p.Name), Phone: p.Phone,
			Display: p.Display, Outcome: OutcomePending})
	}
	run := runs.Run{ID: runs.NewID(), Workflow: ID, Kind: req.Mode, Title: req.GroupName, Subtitle: req.CommunityName,
		Status: runs.StatusRunning, StartedAt: time.Now()}
	r0 := &runState{w: w, run: run, detail: detail}
	if err := r0.save(); err != nil {
		return api.Internal(err)
	}
	title := "Adding people to “" + req.GroupName + "”"
	if req.Mode == "create" {
		title = "Creating “" + req.GroupName + "”"
	}
	snap, err := w.deps.Jobs.Start(jobs.Spec{Workflow: ID, Kind: req.Mode, Title: title, RunID: run.ID}, r0.execute)
	if errors.Is(err, jobs.ErrBusy) {
		r0.run.Status = runs.StatusFailed
		r0.run.Error = "Another task was running, so this one didn't start."
		_ = r0.save()
		return api.Conflict("busy", "The app is busy with another task. Please wait for it to finish, then try again.")
	} else if err != nil {
		return api.Internal(err)
	}
	api.Write(rw, http.StatusOK, map[string]any{"job": snap, "runId": run.ID})
	return nil
}

// loadDetail reads a run's Detail.
func loadDetail(run runs.Run) (Detail, error) {
	var d Detail
	err := json.Unmarshal(run.Detail, &d)
	return d, err
}
