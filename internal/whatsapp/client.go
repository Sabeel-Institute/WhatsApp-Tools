// Package whatsapp defines everything the app asks of WhatsApp, as one
// interface with two implementations: real (whatsmeow) and demo (a realistic
// fake used for trying the app and for automated tests).
//
// Phone numbers cross this boundary in international format with a leading
// "+" (for example "+12125550123"). WhatsApp's own user and group IDs are
// opaque strings: workflows store and pass them back but never show them.
package whatsapp

import (
	"context"
	"time"
)

// Client is the full set of WhatsApp operations. Every method is safe for
// concurrent use. Methods that talk to WhatsApp return *Error values whose
// Kind tells callers whether to wait, retry or give up.
type Client interface {
	// Start opens the session store and, if this computer is already linked,
	// connects in the background. It returns once the status is known.
	Start(ctx context.Context) error
	// Close disconnects (keeping the link) and releases resources.
	Close()

	Status() Status
	// Subscribe delivers status changes and login progress. The returned
	// function unsubscribes and closes the channel.
	Subscribe() (<-chan Event, func())

	// StartQRLogin begins linking with a QR code. Codes, expiry and the
	// outcome arrive as LoginEvents.
	StartQRLogin(ctx context.Context) error
	// StartPhoneLogin begins linking with a pairing code typed on the phone.
	// phone is in international format. The code is also sent as an event.
	StartPhoneLogin(ctx context.Context, phone string) (code string, err error)
	// CancelLogin abandons an in-progress link attempt.
	CancelLogin()
	// Logout unlinks this computer from the WhatsApp account.
	Logout(ctx context.Context) error

	// Communities lists the Communities the account belongs to, each with
	// the linked groups the account has joined.
	Communities(ctx context.Context) ([]Community, error)
	// CheckNumbers reports which numbers have WhatsApp accounts. Callers
	// batch and pace; implementations send one request per call.
	CheckNumbers(ctx context.Context, phones []string) ([]NumberCheck, error)
	// CreateGroup creates an empty group inside a Community.
	CreateGroup(ctx context.Context, req CreateGroupRequest) (Group, error)
	// AddParticipants adds people to a group in one request and reports the
	// outcome for each of them.
	AddParticipants(ctx context.Context, groupID string, people []Participant) ([]AddResult, error)
	// InviteLink returns the group's invite link.
	InviteLink(ctx context.Context, groupID string) (string, error)
	// SendInvite sends one person a personal message inviting them to the
	// group (WhatsApp's own invite card when an invite token exists, or a
	// text message with the link otherwise).
	SendInvite(ctx context.Context, to Participant, invite Invite) error
}

// State is the connection state shown to the user.
type State string

const (
	StateStarting   State = "starting"   // opening the session store
	StateUnlinked   State = "unlinked"   // never linked, or unlinked by the user
	StateLinking    State = "linking"    // a QR code or pairing code is on screen
	StateConnecting State = "connecting" // linked; connecting or reconnecting
	StateConnected  State = "connected"
	StateLoggedOut  State = "logged_out" // unlinked from the phone or by WhatsApp
	StateOutdated   State = "outdated"   // WhatsApp requires a newer app version
	StateBanned     State = "banned"     // WhatsApp temporarily restricted the account
	StateReplaced   State = "replaced"   // the same link was opened elsewhere
)

// Status is a snapshot of the connection.
type Status struct {
	State   State    `json:"state"`
	Account *Account `json:"account,omitempty"`
	// Detail carries state-specific information, such as when a temporary
	// restriction ends. It is already in plain language.
	Detail string    `json:"detail,omitempty"`
	Since  time.Time `json:"since"`
}

// Account is the WhatsApp account this computer is linked to.
type Account struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`   // international format, "+..."
	Display string `json:"display"` // Phone formatted for reading
}

// LoginKind identifies a login progress event.
type LoginKind string

const (
	LoginQR       LoginKind = "qr"        // a new QR code to show
	LoginCode     LoginKind = "pair_code" // a pairing code to type on the phone
	LoginSuccess  LoginKind = "success"   // linked; a status event follows
	LoginExpired  LoginKind = "expired"   // codes ran out; the user can retry
	LoginError    LoginKind = "error"
	LoginCanceled LoginKind = "canceled"
)

// LoginEvent reports progress of a QR or pairing-code login.
type LoginEvent struct {
	Kind LoginKind `json:"kind"`
	// QR is the raw text to encode as a QR code (LoginQR).
	QR string `json:"qr,omitempty"`
	// Code is the pairing code (LoginCode).
	Code string `json:"code,omitempty"`
	// ExpiresIn is how long the current QR code stays valid.
	ExpiresIn time.Duration `json:"expiresIn,omitempty"`
	// Reason is set for LoginError.
	Reason LoginErrorReason `json:"reason,omitempty"`
	// Account is set for LoginSuccess.
	Account *Account `json:"account,omitempty"`
}

// LoginErrorReason is a machine-readable login failure.
type LoginErrorReason string

const (
	LoginErrNetwork         LoginErrorReason = "network"         // can't reach WhatsApp
	LoginErrOutdated        LoginErrorReason = "outdated"        // app needs an update
	LoginErrNoMultiDevice   LoginErrorReason = "no_multi_device" // phone can't link devices
	LoginErrPasskey         LoginErrorReason = "passkey"         // account requires a passkey
	LoginErrBadPhone        LoginErrorReason = "bad_phone"       // pairing number rejected
	LoginErrRateLimited     LoginErrorReason = "rate_limited"    // too many attempts
	LoginErrUnexpected      LoginErrorReason = "unexpected"
	LoginErrAlreadyLinked   LoginErrorReason = "already_linked"
	LoginErrPairingRejected LoginErrorReason = "pairing_failed" // phone or server rejected the link
)

// EventType distinguishes Event payloads.
type EventType string

const (
	EventStatus EventType = "status"
	EventLogin  EventType = "login"
)

// Event is delivered to subscribers.
type Event struct {
	Type   EventType   `json:"type"`
	Status *Status     `json:"status,omitempty"`
	Login  *LoginEvent `json:"login,omitempty"`
}

// Community is a WhatsApp Community with the account's view of it.
type Community struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"isAdmin"`
	// Groups are the linked groups the account has joined, excluding the
	// Community's announcement group.
	Groups []Group `json:"groups"`
}

// Group is a group inside a Community.
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        int    `json:"size"`
	IsAdmin     bool   `json:"isAdmin"`
	CommunityID string `json:"communityId"`
}

// NumberCheck is the result of looking up one phone number.
type NumberCheck struct {
	Phone      string `json:"phone"`
	OnWhatsApp bool   `json:"onWhatsApp"`
	// UserID is WhatsApp's ID for the person, used when adding them.
	UserID string `json:"userId,omitempty"`
}

// CreateGroupRequest describes a new group.
type CreateGroupRequest struct {
	Name             string
	Description      string
	CommunityID      string
	OnlyAdminsSend   bool // "announcement" group
	OnlyAdminsEdit   bool // "locked" group info
	ApprovalRequired bool // joining by link needs admin approval
}

// Participant identifies a person to add or invite.
type Participant struct {
	Phone  string // international format
	UserID string // from CheckNumbers; derived from Phone when empty
}

// AddOutcome is the result of adding one person.
type AddOutcome string

const (
	AddAdded         AddOutcome = "added"
	AddAlreadyMember AddOutcome = "already_member"
	AddPrivacy       AddOutcome = "privacy"       // their settings require an invite
	AddRecentlyLeft  AddOutcome = "recently_left" // left recently; must rejoin by link
	AddNotOnWhatsApp AddOutcome = "not_on_whatsapp"
	AddFailed        AddOutcome = "failed"
)

// AddResult is the outcome for one person.
type AddResult struct {
	Phone   string     `json:"phone"`
	Outcome AddOutcome `json:"outcome"`
	// Code is WhatsApp's numeric status, kept for the log only.
	Code int `json:"code,omitempty"`
	// Invite is set when WhatsApp offers a personal invite for this person.
	Invite *InviteToken `json:"invite,omitempty"`
}

// InviteToken lets an admin send a personal invite to someone whose privacy
// settings prevented adding them.
type InviteToken struct {
	Code    string    `json:"code"`
	Expires time.Time `json:"expires"`
}

// Invite is a personal invitation to a group.
type Invite struct {
	GroupID   string
	GroupName string
	Link      string
	Message   string       // the text the person reads
	Token     *InviteToken // optional; enables WhatsApp's invite card
}

// MaxGroupSize is WhatsApp's member limit for a group, including admins.
// WhatsApp raised it to 1,024 in 2022; whatsmeow does not expose a limit.
const MaxGroupSize = 1024

// MaxGroupNameLength is the subject limit; longer names fail with HTTP 406.
// WhatsApp counts UTF-16 code units, so emoji count as two.
const MaxGroupNameLength = 25

// MaxDescriptionLength is the group description limit.
const MaxDescriptionLength = 2048
