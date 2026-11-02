// Package demo is a realistic stand-in for WhatsApp. It links without a
// phone, invents a few Communities, and gives each phone number a stable
// outcome (most are added; a few are not on WhatsApp, need an invite, or hit
// a temporary error that succeeds on retry), so the whole app can be tried
// and tested safely.
package demo

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/platform"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// Options tune the fake. Zero values give the defaults used by the app.
type Options struct {
	// Latency scales every simulated delay; 0 means 1.0. Tests use a tiny
	// value to run fast.
	Latency float64
	// LinkAfter is how long a QR or pairing code takes to "scan".
	LinkAfter time.Duration
	// QRRefresh is how often a new QR code appears.
	QRRefresh time.Duration
	// OfflineFor is how long Simulate("offline") drops the connection.
	OfflineFor time.Duration
}

// Client is the demo implementation of whatsapp.Client.
type Client struct {
	dir  string
	opts Options
	bus  whatsapp.Broadcaster

	mu          sync.Mutex
	status      whatsapp.Status
	loginGen    int // invalidates pending login timers
	groups      map[string]*demoGroup
	communities []*demoCommunity
	failedOnce  map[string]bool // phones whose temporary error already happened
	nextGroup   int
	offlineTill time.Time
}

type demoCommunity struct {
	id, name string
	isAdmin  bool
	groups   []string
}

type demoGroup struct {
	id, name, communityID string
	isAdmin               bool
	baseSize              int
	members               map[string]bool
	preexisting           bool
}

var account = whatsapp.Account{Name: "Sabeel Front Desk", Phone: "+12125550100"}

// New returns a demo client that remembers whether it is linked in dir.
func New(dir string, opts Options) *Client {
	if opts.Latency == 0 {
		opts.Latency = 1
	}
	if opts.LinkAfter == 0 {
		opts.LinkAfter = 7 * time.Second
	}
	if opts.QRRefresh == 0 {
		opts.QRRefresh = 20 * time.Second
	}
	if opts.OfflineFor == 0 {
		opts.OfflineFor = 20 * time.Second
	}
	c := &Client{
		dir:        dir,
		opts:       opts,
		groups:     map[string]*demoGroup{},
		failedOnce: map[string]bool{},
	}
	c.seed()
	return c
}

func (c *Client) seed() {
	add := func(id, name string, admin bool, groups ...*demoGroup) {
		dc := &demoCommunity{id: id, name: name, isAdmin: admin}
		for _, g := range groups {
			g.communityID = id
			g.members = map[string]bool{}
			g.preexisting = true
			c.groups[g.id] = g
			dc.groups = append(dc.groups, g.id)
		}
		c.communities = append(c.communities, dc)
	}
	add("demo-community-1", "Sabeel Institute", true,
		&demoGroup{id: "demo-g-tafsir", name: "Tafsir Circle Fall 2026", isAdmin: true, baseSize: 38},
		&demoGroup{id: "demo-g-arabic", name: "Arabic Level 1", isAdmin: true, baseSize: 54},
		&demoGroup{id: "demo-g-volunteers", name: "Volunteers Team", isAdmin: false, baseSize: 21},
	)
	add("demo-community-2", "Sabeel Youth", true,
		&demoGroup{id: "demo-g-youth", name: "Youth Halaqa", isAdmin: true, baseSize: 27},
	)
	add("demo-community-3", "Masjid Parents Network", false,
		&demoGroup{id: "demo-g-parents", name: "Parents Chat", isAdmin: false, baseSize: 140},
	)
}

func (c *Client) sessionFile() string { return filepath.Join(c.dir, "demo-session.json") }

// Start implements whatsapp.Client.
func (c *Client) Start(ctx context.Context) error {
	linked := false
	if data, err := os.ReadFile(c.sessionFile()); err == nil {
		var s struct{ Linked bool }
		linked = json.Unmarshal(data, &s) == nil && s.Linked
	}
	if !linked {
		c.setStatus(whatsapp.StateUnlinked, nil, "")
		return nil
	}
	c.setStatus(whatsapp.StateConnecting, nil, "")
	go func() {
		c.sleep(context.Background(), 900*time.Millisecond)
		c.setStatus(whatsapp.StateConnected, &account, "")
	}()
	return nil
}

// Close implements whatsapp.Client.
func (c *Client) Close() {}

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
	c.status = whatsapp.Status{State: state, Account: acct, Detail: detail, Since: time.Now()}
	s := c.status
	c.mu.Unlock()
	c.bus.Publish(whatsapp.Event{Type: whatsapp.EventStatus, Status: &s})
}

func (c *Client) login(e whatsapp.LoginEvent) {
	c.bus.Publish(whatsapp.Event{Type: whatsapp.EventLogin, Login: &e})
}

func (c *Client) sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(time.Duration(float64(d) * c.opts.Latency))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func randomCode(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// StartQRLogin implements whatsapp.Client. Codes rotate like WhatsApp's and
// the "phone" scans after LinkAfter.
func (c *Client) StartQRLogin(ctx context.Context) error {
	c.mu.Lock()
	if c.status.State == whatsapp.StateConnected {
		c.mu.Unlock()
		return whatsapp.NewError(whatsapp.KindInvalid, "qr login", errors.New("already linked"))
	}
	c.loginGen++
	gen := c.loginGen
	c.mu.Unlock()
	c.setStatus(whatsapp.StateLinking, nil, "")

	go func() {
		c.sleep(context.Background(), 600*time.Millisecond)
		deadline := time.Now().Add(c.opts.LinkAfter)
		for {
			if !c.current(gen) {
				return
			}
			c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginQR,
				QR: "2@demo-" + strings.ToLower(randomCode(24)) + ",demo-not-a-real-whatsapp-code", ExpiresIn: c.opts.QRRefresh})
			wait := c.opts.QRRefresh
			if until := time.Until(deadline); until < wait {
				wait = until
			}
			time.Sleep(wait)
			if time.Now().After(deadline) {
				break
			}
		}
		c.completeLink(gen, account)
	}()
	return nil
}

// StartPhoneLogin implements whatsapp.Client.
func (c *Client) StartPhoneLogin(ctx context.Context, phone string) (string, error) {
	if err := c.sleep(ctx, 1200*time.Millisecond); err != nil {
		return "", err
	}
	c.mu.Lock()
	c.loginGen++
	gen := c.loginGen
	c.mu.Unlock()
	c.setStatus(whatsapp.StateLinking, nil, "")
	raw := randomCode(5)
	code := raw[:4] + "-" + raw[4:8]
	c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginCode, Code: code})
	acct := whatsapp.Account{Name: account.Name, Phone: phone}
	go func() {
		time.Sleep(c.opts.LinkAfter + 2*time.Second)
		c.completeLink(gen, acct)
	}()
	return code, nil
}

func (c *Client) current(gen int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return gen == c.loginGen
}

func (c *Client) completeLink(gen int, acct whatsapp.Account) {
	if !c.current(gen) {
		return
	}
	data, _ := json.Marshal(map[string]bool{"Linked": true})
	_ = platform.WriteFileAtomic(c.sessionFile(), data, 0o600)
	c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginSuccess, Account: &acct})
	c.setStatus(whatsapp.StateConnecting, nil, "")
	time.Sleep(time.Duration(float64(800*time.Millisecond) * c.opts.Latency))
	if c.current(gen) {
		c.setStatus(whatsapp.StateConnected, &acct, "")
	}
}

// CancelLogin implements whatsapp.Client.
func (c *Client) CancelLogin() {
	c.mu.Lock()
	c.loginGen++
	linking := c.status.State == whatsapp.StateLinking
	c.mu.Unlock()
	if linking {
		c.login(whatsapp.LoginEvent{Kind: whatsapp.LoginCanceled})
		c.setStatus(whatsapp.StateUnlinked, nil, "")
	}
}

// Logout implements whatsapp.Client.
func (c *Client) Logout(ctx context.Context) error {
	if err := c.sleep(ctx, 600*time.Millisecond); err != nil {
		return err
	}
	_ = os.Remove(c.sessionFile())
	c.mu.Lock()
	c.loginGen++
	c.mu.Unlock()
	c.setStatus(whatsapp.StateUnlinked, nil, "")
	return nil
}

// ready returns an error unless the demo account is connected.
func (c *Client) ready(op string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case c.status.State == whatsapp.StateConnected && time.Now().After(c.offlineTill):
		return nil
	case c.status.State == whatsapp.StateConnecting || time.Now().Before(c.offlineTill):
		return whatsapp.NewError(whatsapp.KindNotConnected, op, errors.New("demo: offline"))
	default:
		return whatsapp.NewError(whatsapp.KindLoggedOut, op, errors.New("demo: not linked"))
	}
}

// Communities implements whatsapp.Client.
func (c *Client) Communities(ctx context.Context) ([]whatsapp.Community, error) {
	if err := c.ready("list communities"); err != nil {
		return nil, err
	}
	if err := c.sleep(ctx, 700*time.Millisecond); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]whatsapp.Community, 0, len(c.communities))
	for _, dc := range c.communities {
		com := whatsapp.Community{ID: dc.id, Name: dc.name, IsAdmin: dc.isAdmin, Groups: []whatsapp.Group{}}
		for _, gid := range dc.groups {
			g := c.groups[gid]
			com.Groups = append(com.Groups, whatsapp.Group{ID: g.id, Name: g.name, Size: g.baseSize + len(g.members),
				IsAdmin: g.isAdmin, CommunityID: dc.id})
		}
		out = append(out, com)
	}
	return out, nil
}

// bucket gives every phone number a stable value in [0, 100).
func bucket(phone string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(phone))
	return int(h.Sum32() % 100)
}

// Outcome buckets, chosen to give a realistic mix.
const (
	notOnWhatsAppBelow = 7  // 7%
	privacyBelow       = 15 // 8%
	temporaryBelow     = 19 // 4%: fails once, then succeeds
	alreadyMemberBelow = 22 // 3%, only for groups that existed before
)

// CheckNumbers implements whatsapp.Client.
func (c *Client) CheckNumbers(ctx context.Context, phones []string) ([]whatsapp.NumberCheck, error) {
	if err := c.ready("check numbers"); err != nil {
		return nil, err
	}
	if err := c.sleep(ctx, time.Duration(400+20*len(phones))*time.Millisecond); err != nil {
		return nil, err
	}
	out := make([]whatsapp.NumberCheck, len(phones))
	for i, p := range phones {
		on := bucket(p) >= notOnWhatsAppBelow
		out[i] = whatsapp.NumberCheck{Phone: p, OnWhatsApp: on}
		if on {
			out[i].UserID = strings.TrimPrefix(p, "+") + "@demo"
		}
	}
	return out, nil
}

// CreateGroup implements whatsapp.Client.
func (c *Client) CreateGroup(ctx context.Context, req whatsapp.CreateGroupRequest) (whatsapp.Group, error) {
	if err := c.ready("create group"); err != nil {
		return whatsapp.Group{}, err
	}
	if n := len(utf16.Encode([]rune(req.Name))); n == 0 || n > whatsapp.MaxGroupNameLength {
		return whatsapp.Group{}, whatsapp.NewError(whatsapp.KindInvalid, "create group", fmt.Errorf("name length %d", n))
	}
	if err := c.sleep(ctx, 1500*time.Millisecond); err != nil {
		return whatsapp.Group{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var com *demoCommunity
	for _, dc := range c.communities {
		if dc.id == req.CommunityID {
			com = dc
		}
	}
	if com == nil {
		return whatsapp.Group{}, whatsapp.NewError(whatsapp.KindNotFound, "create group", errors.New("no such community"))
	}
	if !com.isAdmin {
		return whatsapp.Group{}, whatsapp.NewError(whatsapp.KindNotAdmin, "create group", errors.New("not a community admin"))
	}
	c.nextGroup++
	g := &demoGroup{id: fmt.Sprintf("demo-g-new-%d", c.nextGroup), name: req.Name, communityID: com.id,
		isAdmin: true, baseSize: 1, members: map[string]bool{}}
	c.groups[g.id] = g
	com.groups = append(com.groups, g.id)
	return whatsapp.Group{ID: g.id, Name: g.name, Size: 1, IsAdmin: true, CommunityID: com.id}, nil
}

// AddParticipants implements whatsapp.Client.
func (c *Client) AddParticipants(ctx context.Context, groupID string, people []whatsapp.Participant) ([]whatsapp.AddResult, error) {
	if err := c.ready("add participants"); err != nil {
		return nil, err
	}
	if err := c.sleep(ctx, time.Duration(700+120*len(people))*time.Millisecond); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	g, ok := c.groups[groupID]
	if !ok {
		return nil, whatsapp.NewError(whatsapp.KindNotFound, "add participants", errors.New("no such group"))
	}
	if !g.isAdmin {
		return nil, whatsapp.NewError(whatsapp.KindNotAdmin, "add participants", errors.New("not a group admin"))
	}
	// A temporary server error the first time certain numbers are sent.
	for _, p := range people {
		if b := bucket(p.Phone); b >= privacyBelow && b < temporaryBelow && !c.failedOnce[p.Phone] {
			c.failedOnce[p.Phone] = true
			return nil, whatsapp.NewError(whatsapp.KindTemporary, "add participants", errors.New("demo: server busy (503)"))
		}
	}
	out := make([]whatsapp.AddResult, len(people))
	for i, p := range people {
		r := whatsapp.AddResult{Phone: p.Phone}
		b := bucket(p.Phone)
		switch {
		case g.members[p.Phone]:
			r.Outcome, r.Code = whatsapp.AddAlreadyMember, 409
		case b < notOnWhatsAppBelow:
			r.Outcome, r.Code = whatsapp.AddNotOnWhatsApp, 404
		case b < privacyBelow:
			r.Outcome, r.Code = whatsapp.AddPrivacy, 403
			r.Invite = &whatsapp.InviteToken{Code: randomCode(10), Expires: time.Now().Add(72 * time.Hour)}
		case g.preexisting && b >= temporaryBelow && b < alreadyMemberBelow:
			g.members[p.Phone] = true
			r.Outcome, r.Code = whatsapp.AddAlreadyMember, 409
		default:
			if g.baseSize+len(g.members) >= whatsapp.MaxGroupSize {
				r.Outcome, r.Code = whatsapp.AddFailed, 500
				break
			}
			g.members[p.Phone] = true
			r.Outcome, r.Code = whatsapp.AddAdded, 200
		}
		out[i] = r
	}
	return out, nil
}

// InviteLink implements whatsapp.Client.
func (c *Client) InviteLink(ctx context.Context, groupID string) (string, error) {
	if err := c.ready("invite link"); err != nil {
		return "", err
	}
	if err := c.sleep(ctx, 500*time.Millisecond); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.groups[groupID]; !ok {
		return "", whatsapp.NewError(whatsapp.KindNotFound, "invite link", errors.New("no such group"))
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(groupID))
	return fmt.Sprintf("https://chat.whatsapp.com/Demo%016X", h.Sum64()), nil
}

// SendInvite implements whatsapp.Client.
func (c *Client) SendInvite(ctx context.Context, to whatsapp.Participant, invite whatsapp.Invite) error {
	if err := c.ready("send invite"); err != nil {
		return err
	}
	return c.sleep(ctx, 700*time.Millisecond)
}

// Simulate drives the demo into states that are hard to reach otherwise, so
// they can be tried and screenshotted: "offline" (a network drop lasting
// Options.OfflineFor),
// "logout", "outdated" and "ban".
func (c *Client) Simulate(what string) error {
	switch what {
	case "offline":
		c.mu.Lock()
		c.offlineTill = time.Now().Add(c.opts.OfflineFor)
		acct := c.status.Account
		c.mu.Unlock()
		c.setStatus(whatsapp.StateConnecting, acct, "")
		go func() {
			time.Sleep(c.opts.OfflineFor)
			c.setStatus(whatsapp.StateConnected, acct, "")
		}()
	case "logout":
		_ = os.Remove(c.sessionFile())
		c.setStatus(whatsapp.StateLoggedOut, nil, "")
	case "outdated":
		c.setStatus(whatsapp.StateOutdated, nil, "")
	case "ban":
		c.setStatus(whatsapp.StateBanned, nil, "about 24 hours")
	default:
		return fmt.Errorf("unknown simulation %q", what)
	}
	return nil
}

var _ whatsapp.Client = (*Client)(nil)
