// Package real implements whatsapp.Client with whatsmeow, linking this
// computer to a WhatsApp account as a "linked device". The session (login
// keys) is stored in a SQLite database through a pure-Go driver, so the app
// builds without CGO for every platform.
package real

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	phonefmt "github.com/Sabeel-Institute/WhatsApp-Tools/internal/phone"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// DeviceName is how this computer appears in WhatsApp > Linked devices.
const DeviceName = "Sabeel WhatsApp Tools"

// Client is the whatsmeow implementation of whatsapp.Client.
type Client struct {
	dbPath  string
	log     *slog.Logger
	waLog   slog.Level
	version [3]uint32

	container *sqlstore.Container
	bus       whatsapp.Broadcaster

	mu          sync.Mutex
	cli         *whatsmeow.Client
	handlerID   uint32
	status      whatsapp.Status
	loginCancel context.CancelFunc
	loginGen    int
	refreshedWA bool // the WhatsApp Web version was refreshed after "outdated"
}

// New returns a client storing its session in dataDir/session.db.
// appVersion is shown on the phone next to DeviceName.
func New(dataDir string, log *slog.Logger, appVersion [3]uint32) *Client {
	return &Client{
		dbPath:  filepath.Join(dataDir, "session.db"),
		log:     log.With("component", "whatsapp"),
		waLog:   slog.LevelInfo,
		version: appVersion,
		status:  whatsapp.Status{State: whatsapp.StateStarting, Since: time.Now()},
	}
}

// Start implements whatsapp.Client.
func (c *Client) Start(ctx context.Context) error {
	store.SetOSInfo(DeviceName, c.version)
	store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_DESKTOP.Enum()

	// Create the database owner-only before SQLite opens it; SQLite gives
	// its WAL and shared-memory files the same permissions.
	f, err := os.OpenFile(c.dbPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("creating session database: %w", err)
	}
	_ = f.Close()
	if err := os.Chmod(c.dbPath, 0o600); err != nil {
		return fmt.Errorf("restricting session database: %w", err)
	}

	dsn := c.dbPath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	container, err := sqlstore.New(ctx, "sqlite", dsn, newLogger(c.log, slog.LevelWarn))
	if err != nil {
		return fmt.Errorf("opening session database: %w", err)
	}
	c.container = container
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("loading session: %w", err)
	}
	c.mu.Lock()
	c.useDevice(device)
	linked := device.ID != nil
	c.mu.Unlock()

	if !linked {
		c.setStatus(whatsapp.StateUnlinked, nil, "")
		return nil
	}
	c.setStatus(whatsapp.StateConnecting, c.account(), "")
	go c.connect()
	return nil
}

// useDevice builds a whatsmeow client around device. Caller holds c.mu.
func (c *Client) useDevice(device *store.Device) {
	if c.cli != nil {
		c.cli.RemoveEventHandler(c.handlerID)
		c.cli.Disconnect()
	}
	cli := whatsmeow.NewClient(device, newLogger(c.log, c.waLog))
	cli.EnableAutoReconnect = true
	cli.InitialAutoReconnect = true
	c.cli = cli
	c.handlerID = cli.AddEventHandler(func(evt any) { c.handleEvent(cli, evt) })
}

func (c *Client) client() *whatsmeow.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cli
}

func (c *Client) connect() {
	cli := c.client()
	if err := cli.Connect(); err != nil && !errors.Is(err, whatsmeow.ErrAlreadyConnected) {
		c.log.Warn("connect failed", "err", err)
		c.setStatus(whatsapp.StateConnecting, c.account(), "")
	}
}

// Close implements whatsapp.Client.
func (c *Client) Close() {
	c.mu.Lock()
	if c.loginCancel != nil {
		c.loginCancel()
	}
	cli := c.cli
	c.mu.Unlock()
	if cli != nil {
		cli.Disconnect()
	}
	if c.container != nil {
		_ = c.container.Close()
	}
}

// Status implements whatsapp.Client.
func (c *Client) Status() whatsapp.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// Subscribe implements whatsapp.Client.
func (c *Client) Subscribe() (<-chan whatsapp.Event, func()) { return c.bus.Subscribe() }

func (c *Client) setStatus(state whatsapp.State, acct *whatsapp.Account, detail string) {
	c.mu.Lock()
	if c.status.State == state && c.status.Detail == detail && sameAccount(c.status.Account, acct) {
		c.mu.Unlock()
		return
	}
	c.status = whatsapp.Status{State: state, Account: acct, Detail: detail, Since: time.Now()}
	s := c.status
	c.mu.Unlock()
	c.log.Info("status", "state", state)
	c.bus.Publish(whatsapp.Event{Type: whatsapp.EventStatus, Status: &s})
}

func sameAccount(a, b *whatsapp.Account) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (c *Client) login(e whatsapp.LoginEvent) {
	c.bus.Publish(whatsapp.Event{Type: whatsapp.EventLogin, Login: &e})
}

// account describes the linked account, or nil when not linked.
func (c *Client) account() *whatsapp.Account {
	cli := c.client()
	if cli == nil || cli.Store.ID == nil {
		return nil
	}
	name := cli.Store.PushName
	if name == "" {
		name = cli.Store.BusinessName
	}
	phone := "+" + cli.Store.ID.User
	return &whatsapp.Account{Name: name, Phone: phone, Display: phonefmt.Pretty(phone)}
}

func (c *Client) handleEvent(cli *whatsmeow.Client, evt any) {
	if c.client() != cli {
		return // an event from a client replaced after logout
	}
	switch e := evt.(type) {
	case *events.Connected, *events.PushNameSetting:
		c.setStatus(whatsapp.StateConnected, c.account(), "")
	case *events.Disconnected, *events.KeepAliveTimeout:
		if st := c.Status().State; st == whatsapp.StateConnected || st == whatsapp.StateConnecting {
			c.setStatus(whatsapp.StateConnecting, c.account(), "")
		}
	case *events.KeepAliveRestored:
		c.setStatus(whatsapp.StateConnected, c.account(), "")
	case *events.LoggedOut:
		c.log.Warn("logged out by WhatsApp or from the phone", "reason", e.Reason.String())
		go c.resetAfterLogout(whatsapp.StateLoggedOut)
	case *events.StreamReplaced:
		c.setStatus(whatsapp.StateReplaced, c.account(), "")
	case *events.ClientOutdated:
		go c.handleOutdated()
	case *events.TemporaryBan:
		detail := ""
		if e.Expire > 0 {
			detail = humanDuration(e.Expire)
		}
		c.setStatus(whatsapp.StateBanned, c.account(), detail)
	case *events.ConnectFailure:
		c.log.Warn("connect failure", "reason", e.Reason.String())
	}
}

// resetAfterLogout replaces the deleted device with a fresh one so the user
// can link again.
func (c *Client) resetAfterLogout(state whatsapp.State) {
	c.mu.Lock()
	c.useDevice(c.container.NewDevice())
	c.mu.Unlock()
	c.setStatus(state, nil, "")
}

// handleOutdated refreshes the WhatsApp Web version once and reconnects;
// if WhatsApp still refuses, the user needs a newer build of the app.
func (c *Client) handleOutdated() {
	c.mu.Lock()
	tried := c.refreshedWA
	c.refreshedWA = true
	c.mu.Unlock()
	if !tried {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if v, err := whatsmeow.GetLatestVersion(ctx, nil); err == nil && *v != store.GetWAVersion() {
			c.log.Info("updating WhatsApp Web version after outdated error", "version", v.String())
			store.SetWAVersion(*v)
			if c.client().Store.ID != nil {
				c.setStatus(whatsapp.StateConnecting, c.account(), "")
				go c.connect()
			}
			return
		} else if err != nil {
			c.log.Warn("could not fetch the latest WhatsApp Web version", "err", err)
		}
	}
	c.setStatus(whatsapp.StateOutdated, c.account(), "")
}

func humanDuration(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("about %d days", int(d.Hours()/24+0.5))
	case d >= 2*time.Hour:
		return fmt.Sprintf("about %d hours", int(d.Hours()+0.5))
	case d >= 2*time.Minute:
		return fmt.Sprintf("about %d minutes", int(d.Minutes()+0.5))
	}
	return "a few minutes"
}

// ---- linking -----------------------------------------------------------

// beginLogin opens a fresh QR channel and connects. Codes flow to the UI
// unless phoneMode is set, in which case the first code only signals that
// the connection is ready for PairPhone.
func (c *Client) beginLogin(phoneMode bool) (ready <-chan struct{}, gen int, err error) {
	c.mu.Lock()
	if c.loginCancel != nil {
		c.loginCancel()
	}
	if c.cli.Store.ID != nil {
		c.mu.Unlock()
		return nil, 0, whatsapp.NewError(whatsapp.KindInvalid, "login", errors.New("already linked"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.loginCancel = cancel
	c.loginGen++
	gen = c.loginGen
	cli := c.cli
	c.mu.Unlock()

	cli.Disconnect()
	qr, err := cli.GetQRChannel(ctx)
	if err != nil {
		cancel()
		return nil, 0, classify("login", err)
	}
	c.setStatus(whatsapp.StateLinking, nil, "")
	if err := cli.Connect(); err != nil {
		cancel()
		c.setStatus(whatsapp.StateUnlinked, nil, "")
		return nil, 0, whatsapp.NewError(whatsapp.KindNotConnected, "login", err)
	}
	readyCh := make(chan struct{})
	go c.consumeQR(gen, qr, phoneMode, readyCh)
	return readyCh, gen, nil
}

func (c *Client) consumeQR(gen int, qr <-chan whatsmeow.QRChannelItem, phoneMode bool, ready chan struct{}) {
	signalled := false
	current := func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.loginGen == gen
	}
	fail := func(reason whatsapp.LoginErrorReason) {
		if current() {
			c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginError, Reason: reason})
			c.client().Disconnect()
			c.setStatus(whatsapp.StateUnlinked, nil, "")
		}
	}
	for item := range qr {
		if !current() {
			continue // drain a superseded channel
		}
		switch item.Event {
		case whatsmeow.QRChannelEventCode:
			if !signalled {
				signalled = true
				close(ready)
			}
			if !phoneMode {
				c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginQR, QR: item.Code, ExpiresIn: item.Timeout})
			}
		case whatsmeow.QRChannelSuccess.Event:
			c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginSuccess, Account: c.account()})
			c.setStatus(whatsapp.StateConnecting, c.account(), "")
		case whatsmeow.QRChannelTimeout.Event:
			if current() {
				c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginExpired})
				c.setStatus(whatsapp.StateUnlinked, nil, "")
			}
		case whatsmeow.QRChannelClientOutdated.Event:
			go c.handleOutdated()
			fail(whatsapp.LoginErrOutdated)
		case whatsmeow.QRChannelScannedWithoutMultidevice.Event:
			// The same code still works once the phone is updated.
			c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginError, Reason: whatsapp.LoginErrNoMultiDevice})
		case whatsmeow.QRChannelEventPasskeyRequest, whatsmeow.QRChannelEventPasskeyResponse:
			// Passkey confirmation needs a WebAuthn ceremony on whatsapp.com,
			// which a local app cannot perform.
			fail(whatsapp.LoginErrPasskey)
		case whatsmeow.QRChannelErrUnexpectedEvent.Event:
			fail(whatsapp.LoginErrUnexpected)
		case whatsmeow.QRChannelEventError:
			c.log.Warn("pairing failed", "err", item.Error)
			fail(whatsapp.LoginErrPairingRejected)
		}
	}
	if !signalled {
		close(ready)
	}
}

// StartQRLogin implements whatsapp.Client.
func (c *Client) StartQRLogin(ctx context.Context) error {
	_, _, err := c.beginLogin(false)
	return err
}

// pairDisplayName is the "Browser (OS)" label WhatsApp requires for
// pairing-code logins; the server only accepts common names.
func pairDisplayName() string {
	switch runtime.GOOS {
	case "windows":
		return "Chrome (Windows)"
	case "darwin":
		return "Chrome (Mac OS)"
	}
	return "Chrome (Linux)"
}

// StartPhoneLogin implements whatsapp.Client.
func (c *Client) StartPhoneLogin(ctx context.Context, phone string) (string, error) {
	ready, gen, err := c.beginLogin(true)
	if err != nil {
		return "", err
	}
	select {
	case <-ready:
	case <-ctx.Done():
		c.CancelLogin()
		return "", ctx.Err()
	case <-time.After(30 * time.Second):
		c.CancelLogin()
		return "", whatsapp.NewError(whatsapp.KindNotConnected, "pair phone", errors.New("timed out waiting for WhatsApp"))
	}
	code, err := c.client().PairPhone(ctx, strings.TrimPrefix(phone, "+"), true, whatsmeow.PairClientChrome, pairDisplayName())
	if err != nil {
		c.CancelLogin()
		switch {
		case errors.Is(err, whatsmeow.ErrPhoneNumberTooShort), errors.Is(err, whatsmeow.ErrPhoneNumberIsNotInternational),
			errors.Is(err, whatsmeow.ErrIQBadRequest):
			return "", whatsapp.NewError(whatsapp.KindInvalid, "pair phone", err)
		}
		return "", classify("pair phone", err)
	}
	c.mu.Lock()
	stillCurrent := c.loginGen == gen
	c.mu.Unlock()
	if stillCurrent {
		c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginCode, Code: code})
	}
	return code, nil
}

// CancelLogin implements whatsapp.Client.
func (c *Client) CancelLogin() {
	c.mu.Lock()
	cancel := c.loginCancel
	c.loginCancel = nil
	c.loginGen++
	linked := c.cli.Store.ID != nil
	cli := c.cli
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if !linked {
		cli.Disconnect()
		c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginCanceled})
		c.setStatus(whatsapp.StateUnlinked, nil, "")
	}
}

// Logout implements whatsapp.Client. If WhatsApp can't be reached the link
// is removed locally anyway, and the phone shows a stale linked device until
// the user removes it there.
func (c *Client) Logout(ctx context.Context) error {
	cli := c.client()
	if cli.Store.ID == nil {
		c.setStatus(whatsapp.StateUnlinked, nil, "")
		return nil
	}
	if err := cli.Logout(ctx); err != nil {
		c.log.Warn("logout request failed; removing the session locally", "err", err)
		cli.Disconnect()
		if derr := cli.Store.Delete(ctx); derr != nil {
			return fmt.Errorf("removing local session: %w", derr)
		}
		c.resetAfterLogout(whatsapp.StateUnlinked)
		return classify("logout", err)
	}
	c.resetAfterLogout(whatsapp.StateUnlinked)
	return nil
}

// ---- groups ------------------------------------------------------------

func (c *Client) ready(op string) (*whatsmeow.Client, error) {
	cli := c.client()
	if cli.Store.ID == nil {
		return nil, whatsapp.NewError(whatsapp.KindLoggedOut, op, whatsmeow.ErrNotLoggedIn)
	}
	if !cli.IsConnected() || !cli.IsLoggedIn() {
		return nil, whatsapp.NewError(whatsapp.KindNotConnected, op, whatsmeow.ErrNotConnected)
	}
	return cli, nil
}

// isSelf reports whether jid is the linked account (by phone number or by
// WhatsApp's newer anonymous "LID" identifier).
func isSelf(cli *whatsmeow.Client, p types.GroupParticipant) bool {
	own, lid := cli.Store.GetJID(), cli.Store.GetLID()
	for _, j := range []types.JID{p.JID, p.PhoneNumber, p.LID} {
		if j.IsEmpty() {
			continue
		}
		if (!own.IsEmpty() && j.User == own.User && j.Server == own.Server) ||
			(!lid.IsEmpty() && j.User == lid.User && j.Server == lid.Server) {
			return true
		}
	}
	return false
}

func selfIsAdmin(cli *whatsmeow.Client, g *types.GroupInfo) bool {
	for _, p := range g.Participants {
		if isSelf(cli, p) {
			return p.IsAdmin || p.IsSuperAdmin
		}
	}
	return false
}

func groupSize(g *types.GroupInfo) int {
	if g.ParticipantCount > 0 {
		return g.ParticipantCount
	}
	return len(g.Participants)
}

// standaloneID groups the account's admin groups that are not inside any
// Community, so late registrants can be added to those too.
const standaloneID = "standalone"

// Communities implements whatsapp.Client.
func (c *Client) Communities(ctx context.Context) ([]whatsapp.Community, error) {
	cli, err := c.ready("list communities")
	if err != nil {
		return nil, err
	}
	groups, err := cli.GetJoinedGroups(ctx)
	if err != nil {
		return nil, classify("list communities", err)
	}
	byID := map[string]*whatsapp.Community{}
	var order []string
	ensure := func(jid types.JID) *whatsapp.Community {
		id := jid.String()
		if com, ok := byID[id]; ok {
			return com
		}
		com := &whatsapp.Community{ID: id, Groups: []whatsapp.Group{}}
		byID[id] = com
		order = append(order, id)
		return com
	}
	for _, g := range groups {
		switch {
		case g.IsParent:
			com := ensure(g.JID)
			com.Name = g.Name
			com.IsAdmin = com.IsAdmin || selfIsAdmin(cli, g)
		case g.IsDefaultSubGroup && !g.LinkedParentJID.IsEmpty():
			// Community admins administer its announcement group too.
			com := ensure(g.LinkedParentJID)
			com.IsAdmin = com.IsAdmin || selfIsAdmin(cli, g)
		case !g.LinkedParentJID.IsEmpty():
			com := ensure(g.LinkedParentJID)
			com.Groups = append(com.Groups, whatsapp.Group{ID: g.JID.String(), Name: g.Name,
				Size: groupSize(g), IsAdmin: selfIsAdmin(cli, g), CommunityID: com.ID})
		default:
			if selfIsAdmin(cli, g) {
				com := ensure(types.NewJID(standaloneID, types.GroupServer))
				com.Groups = append(com.Groups, whatsapp.Group{ID: g.JID.String(), Name: g.Name,
					Size: groupSize(g), IsAdmin: true, CommunityID: com.ID})
			}
		}
	}
	out := make([]whatsapp.Community, 0, len(order))
	for _, id := range order {
		com := byID[id]
		if jid, _ := types.ParseJID(id); jid.User == standaloneID {
			com.ID = standaloneID
			com.Name = "Groups outside a Community"
			for i := range com.Groups {
				com.Groups[i].CommunityID = standaloneID
			}
		} else if com.Name == "" {
			// The account is in a linked group but not the Community itself.
			if info, err := cli.GetGroupInfo(ctx, jid); err == nil {
				com.Name = info.Name
			} else {
				com.Name = "Community"
			}
		}
		out = append(out, *com)
	}
	return out, nil
}

// CheckNumbers implements whatsapp.Client.
func (c *Client) CheckNumbers(ctx context.Context, phones []string) ([]whatsapp.NumberCheck, error) {
	cli, err := c.ready("check numbers")
	if err != nil {
		return nil, err
	}
	resp, err := cli.IsOnWhatsApp(ctx, phones)
	if err != nil {
		return nil, classify("check numbers", err)
	}
	found := make(map[string]types.IsOnWhatsAppResponse, len(resp))
	for _, r := range resp {
		found[digitsOnly(r.Query)] = r
	}
	out := make([]whatsapp.NumberCheck, len(phones))
	for i, p := range phones {
		out[i] = whatsapp.NumberCheck{Phone: p}
		if r, ok := found[digitsOnly(p)]; ok && r.IsIn {
			out[i].OnWhatsApp = true
			out[i].UserID = r.JID.String()
		}
	}
	return out, nil
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseGroupID(op, id string) (types.JID, error) {
	jid, err := types.ParseJID(id)
	if err != nil || jid.Server != types.GroupServer {
		return types.JID{}, whatsapp.NewError(whatsapp.KindNotFound, op, fmt.Errorf("bad group id: %v", err))
	}
	return jid, nil
}

// CreateGroup implements whatsapp.Client. The group starts empty; people are
// added afterwards in paced batches so every add has a per-person result.
func (c *Client) CreateGroup(ctx context.Context, req whatsapp.CreateGroupRequest) (whatsapp.Group, error) {
	cli, err := c.ready("create group")
	if err != nil {
		return whatsapp.Group{}, err
	}
	parent, err := parseGroupID("create group", req.CommunityID)
	if err != nil {
		return whatsapp.Group{}, err
	}
	info, err := cli.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:                        req.Name,
		GroupAnnounce:               types.GroupAnnounce{IsAnnounce: req.OnlyAdminsSend},
		GroupLocked:                 types.GroupLocked{IsLocked: req.OnlyAdminsEdit},
		GroupMembershipApprovalMode: types.GroupMembershipApprovalMode{IsJoinApprovalRequired: req.ApprovalRequired},
		GroupLinkedParent:           types.GroupLinkedParent{LinkedParentJID: parent},
	})
	if err != nil {
		return whatsapp.Group{}, classify("create group", err)
	}
	g := whatsapp.Group{ID: info.JID.String(), Name: info.Name, Size: max(groupSize(info), 1), IsAdmin: true,
		CommunityID: req.CommunityID}
	if req.Description != "" {
		if err := cli.SetGroupTopic(ctx, info.JID, "", "", req.Description); err != nil {
			c.log.Warn("setting group description failed", "err", err)
			return g, &whatsapp.DescriptionError{Err: classify("set description", err)}
		}
	}
	return g, nil
}

func participantJID(p whatsapp.Participant) types.JID {
	if p.UserID != "" {
		if jid, err := types.ParseJID(p.UserID); err == nil {
			return jid
		}
	}
	return types.NewJID(digitsOnly(p.Phone), types.DefaultUserServer)
}

// AddParticipants implements whatsapp.Client.
func (c *Client) AddParticipants(ctx context.Context, groupID string, people []whatsapp.Participant) ([]whatsapp.AddResult, error) {
	cli, err := c.ready("add participants")
	if err != nil {
		return nil, err
	}
	group, err := parseGroupID("add participants", groupID)
	if err != nil {
		return nil, err
	}
	jids := make([]types.JID, len(people))
	index := map[string]int{} // "user@server" -> position in people
	for i, p := range people {
		jids[i] = participantJID(p)
		index[jids[i].ToNonAD().String()] = i
		index[types.NewJID(digitsOnly(p.Phone), types.DefaultUserServer).String()] = i
	}
	resp, err := cli.UpdateGroupParticipants(ctx, group, jids, whatsmeow.ParticipantChangeAdd)
	if err != nil {
		return nil, classify("add participants", err)
	}
	out := make([]whatsapp.AddResult, len(people))
	seen := make([]bool, len(people))
	for _, r := range resp {
		i, ok := -1, false
		for _, j := range []types.JID{r.JID, r.PhoneNumber, r.LID} {
			if j.IsEmpty() {
				continue
			}
			if i, ok = index[j.ToNonAD().String()]; ok {
				break
			}
		}
		if !ok || seen[i] {
			c.log.Warn("unmatched participant in add response")
			continue
		}
		seen[i] = true
		res := whatsapp.AddResult{Phone: people[i].Phone, Outcome: outcomeOf(r.Error), Code: r.Error}
		if r.AddRequest != nil && r.AddRequest.Code != "" {
			res.Invite = &whatsapp.InviteToken{Code: r.AddRequest.Code, Expires: r.AddRequest.Expiration}
		}
		out[i] = res
	}
	for i := range out {
		if !seen[i] {
			// WhatsApp answered without mentioning this person; count it as
			// a failure the user can retry rather than claim success.
			out[i] = whatsapp.AddResult{Phone: people[i].Phone, Outcome: whatsapp.AddFailed}
		}
	}
	return out, nil
}

// InviteLink implements whatsapp.Client.
func (c *Client) InviteLink(ctx context.Context, groupID string) (string, error) {
	cli, err := c.ready("invite link")
	if err != nil {
		return "", err
	}
	group, err := parseGroupID("invite link", groupID)
	if err != nil {
		return "", err
	}
	link, err := cli.GetGroupInviteLink(ctx, group, false)
	if err != nil {
		return "", classify("invite link", err)
	}
	return link, nil
}

// SendInvite implements whatsapp.Client.
func (c *Client) SendInvite(ctx context.Context, to whatsapp.Participant, invite whatsapp.Invite) error {
	cli, err := c.ready("send invite")
	if err != nil {
		return err
	}
	var msg *waE2E.Message
	if t := invite.Token; t != nil && time.Now().Before(t.Expires) {
		msg = &waE2E.Message{GroupInviteMessage: &waE2E.GroupInviteMessage{
			GroupJID:         proto.String(invite.GroupID),
			InviteCode:       proto.String(t.Code),
			InviteExpiration: proto.Int64(t.Expires.Unix()),
			GroupName:        proto.String(invite.GroupName),
			Caption:          proto.String(invite.Message),
		}}
	} else {
		msg = &waE2E.Message{Conversation: proto.String(invite.Message)}
	}
	if _, err := cli.SendMessage(ctx, participantJID(to), msg); err != nil {
		return classify("send invite", err)
	}
	return nil
}

var _ whatsapp.Client = (*Client)(nil)
