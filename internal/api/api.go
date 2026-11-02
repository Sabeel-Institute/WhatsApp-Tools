// Package api holds the JSON conventions shared by the server and the
// workflows: request decoding, responses, and errors that carry a message
// written for the person using the app.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// Error is an API failure. Message is shown to the user as-is, so it says
// what happened and what to do next, in plain words.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"` // logged, never sent
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

// BadRequest is a 400 with a user-facing message.
func BadRequest(code, message string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: code, Message: message}
}

// Conflict is a 409 with a user-facing message.
func Conflict(code, message string) *Error {
	return &Error{Status: http.StatusConflict, Code: code, Message: message}
}

// NotFound is a 404 with a user-facing message.
func NotFound(message string) *Error {
	return &Error{Status: http.StatusNotFound, Code: "not_found", Message: message}
}

// Internal wraps an unexpected error.
func Internal(err error) *Error {
	return &Error{Status: http.StatusInternalServerError, Code: "internal",
		Message: "Something went wrong inside the app. Please try again; if it keeps happening, restart the app.",
		Err:     err}
}

// WhatsAppMessage explains a WhatsApp failure in plain words.
func WhatsAppMessage(kind whatsapp.Kind) string {
	switch kind {
	case whatsapp.KindNotConnected:
		return "We can't reach WhatsApp right now. Check that this computer is online; the app reconnects by itself."
	case whatsapp.KindRateLimited:
		return "WhatsApp asked us to slow down. Please wait a few minutes and try again."
	case whatsapp.KindTemporary:
		return "WhatsApp didn't respond in time. Please try again in a moment."
	case whatsapp.KindNotAdmin:
		return "The connected WhatsApp account isn't an admin there, so WhatsApp didn't allow it. Ask an admin to make this account an admin, then try again."
	case whatsapp.KindNotFound:
		return "WhatsApp couldn't find that group or Community. It may have been deleted, or this account may have left it."
	case whatsapp.KindInvalid:
		return "WhatsApp didn't accept that request. Please check the details and try again."
	case whatsapp.KindLoggedOut:
		return "This computer is no longer connected to WhatsApp. Please connect again."
	case whatsapp.KindGroupFull:
		return "The group is full. WhatsApp groups can have at most 1,024 members."
	}
	return "WhatsApp returned an unexpected error. Please try again; if it keeps happening, restart the app."
}

// FromWhatsApp converts a whatsapp.Client error into an API error.
func FromWhatsApp(err error) *Error {
	kind := whatsapp.KindOf(err)
	status := http.StatusBadGateway
	switch kind {
	case whatsapp.KindNotConnected, whatsapp.KindLoggedOut:
		status = http.StatusServiceUnavailable
	case whatsapp.KindRateLimited:
		status = http.StatusTooManyRequests
	case whatsapp.KindNotAdmin:
		status = http.StatusForbidden
	case whatsapp.KindNotFound:
		status = http.StatusNotFound
	case whatsapp.KindInvalid:
		status = http.StatusBadRequest
	}
	return &Error{Status: status, Code: "whatsapp_" + string(kind), Message: WhatsAppMessage(kind), Err: err}
}

// maxBody caps JSON request bodies; spreadsheets use their own limit.
const maxBody = 8 << 20

// Decode reads a JSON body into v.
func Decode(r *http.Request, v any) *Error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &Error{Status: http.StatusBadRequest, Code: "bad_json",
			Message: "The app sent a request it couldn't read. Please reload the page.", Err: err}
	}
	return nil
}

// Write sends v as JSON with status.
func Write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Fail sends err, logging anything unexpected.
func Fail(w http.ResponseWriter, log *slog.Logger, err error) {
	var e *Error
	if !errors.As(err, &e) {
		e = Internal(err)
	}
	if e.Status >= 500 || e.Err != nil {
		log.Warn("request failed", "code", e.Code, "err", e.Err)
	}
	Write(w, e.Status, map[string]*Error{"error": e})
}

// HandlerFunc is an http handler that returns an error.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Handle adapts h to http.HandlerFunc, writing errors with Fail.
func Handle(log *slog.Logger, h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			Fail(w, log, err)
		}
	}
}

// Router is what workflows use to register their endpoints. Patterns follow
// net/http.ServeMux ("POST /run", "GET /runs/{id}") and are mounted under
// /api/workflows/<workflow-id>/.
type Router interface {
	Handle(pattern string, h HandlerFunc)
}
