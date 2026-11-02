package whatsapp

import (
	"errors"
	"fmt"
)

// Kind classifies failures by what the caller should do about them.
type Kind string

const (
	// KindNotConnected: wait for the connection to come back, then retry.
	KindNotConnected Kind = "not_connected"
	// KindRateLimited: WhatsApp asked us to slow down; wait longer.
	KindRateLimited Kind = "rate_limited"
	// KindTemporary: a server hiccup or timeout; retry with backoff.
	KindTemporary Kind = "temporary"
	// KindNotAdmin: the account lacks permission for this group or Community.
	KindNotAdmin Kind = "not_admin"
	// KindNotFound: the group or Community no longer exists.
	KindNotFound Kind = "not_found"
	// KindInvalid: WhatsApp rejected the request (for example a name too long).
	KindInvalid Kind = "invalid"
	// KindLoggedOut: this computer is no longer linked.
	KindLoggedOut Kind = "logged_out"
	// KindGroupFull: the group reached its member limit.
	KindGroupFull Kind = "group_full"
	// KindUnknown: anything else; treated as permanent.
	KindUnknown Kind = "unknown"
)

// Error wraps a WhatsApp failure with its Kind.
type Error struct {
	Kind Kind
	Op   string // what we were doing, for the log
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Op, e.Kind)
	}
	return fmt.Sprintf("%s: %s: %v", e.Op, e.Kind, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// NewError builds an *Error.
func NewError(kind Kind, op string, err error) *Error {
	return &Error{Kind: kind, Op: op, Err: err}
}

// KindOf returns the Kind of err, or KindUnknown.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindUnknown
}

// Retryable reports whether waiting and trying again may succeed.
func Retryable(k Kind) bool {
	return k == KindNotConnected || k == KindRateLimited || k == KindTemporary
}

// ErrNotLinked is returned by operations that need a linked account.
var ErrNotLinked = NewError(KindLoggedOut, "check link", errors.New("this computer is not linked to WhatsApp"))

// DescriptionError is returned by CreateGroup when the group was created but
// its description could not be set. Callers treat it as a warning; the
// returned Group is valid.
type DescriptionError struct{ Err error }

func (e *DescriptionError) Error() string {
	return "group created, but the description was not set: " + e.Err.Error()
}

func (e *DescriptionError) Unwrap() error { return e.Err }
