// Package workflows defines how a task such as "Course groups" plugs into
// the app. The host provides WhatsApp, the job runner, run history,
// settings and the shared people/spreadsheet layer; a workflow adds its own
// endpoints and job logic, and the web UI adds its screens under the same ID.
//
// To add a workflow: implement Workflow in a new package under
// internal/workflows/, append it in internal/app, and register its screens
// in web/src/workflows/registry.ts.
package workflows

import (
	"log/slog"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/api"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/people"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/runs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/settings"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// Action is an entry point shown on the home screen.
type Action struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Primary     bool   `json:"primary"`
	Icon        string `json:"icon"` // a Lucide icon name known to the UI
}

// Info describes a workflow to the UI.
type Info struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Actions     []Action `json:"actions"`
}

// Deps are the host services a workflow may use.
type Deps struct {
	WA       whatsapp.Client
	Jobs     *jobs.Manager
	Runs     *runs.Store
	Settings *settings.Store
	Checker  *people.Checker
	Pacing   whatsapp.Pacing
	Log      *slog.Logger
}

// Workflow is one task the app can perform.
type Workflow interface {
	Info() Info
	// Routes registers the workflow's endpoints, mounted under
	// /api/workflows/<id>/.
	Routes(r api.Router)
}
