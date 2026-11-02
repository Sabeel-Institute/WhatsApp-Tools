package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/people"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/runs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/settings"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp/demo"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/workflows"
)

const token = "test-token-123456"

func startServer(t *testing.T) (base string) {
	t.Helper()
	dir := t.TempDir()
	wa := demo.New(dir, demo.Options{Latency: 0.01})
	if err := wa.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	rs, _ := runs.Open(dir)
	st, _ := settings.Open(dir)
	deps := workflows.Deps{WA: wa, Jobs: jobs.NewManager(), Runs: rs, Settings: st,
		Checker: people.NewChecker(wa, whatsapp.DefaultPacing), Pacing: whatsapp.DefaultPacing,
		Log: slog.New(slog.DiscardHandler)}
	static := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>t</title>")}}
	s := New(Config{AppName: "t", Token: token, Static: static, Log: deps.Log}, deps, nil)
	ln, err := s.Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	return "http://" + ln.Addr().String()
}

func do(t *testing.T, method, url string, headers map[string]string, host string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestAPIRequiresTheSessionToken(t *testing.T) {
	base := startServer(t)
	if code, _ := do(t, "GET", base+"/api/app", nil, ""); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
	if code, _ := do(t, "GET", base+"/api/app", map[string]string{tokenHeader: "wrong"}, ""); code != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", code)
	}
	if code, body := do(t, "GET", base+"/api/app", map[string]string{tokenHeader: token}, ""); code != http.StatusOK {
		t.Errorf("right token: %d %s", code, body)
	}
	// The page itself holds no secrets and loads without the token.
	code, body := do(t, "GET", base+"/", nil, "")
	if code != http.StatusOK || !strings.Contains(body, "<title>t</title>") {
		t.Errorf("index: %d %q", code, body)
	}
}

func TestRejectsOtherHostsAndSites(t *testing.T) {
	base := startServer(t)
	port := base[strings.LastIndex(base, ":")+1:]
	ok := map[string]string{tokenHeader: token}

	// DNS rebinding: our port, someone else's host name.
	if code, _ := do(t, "GET", base+"/api/app", ok, "attacker.example:"+port); code != http.StatusForbidden {
		t.Errorf("foreign host: %d", code)
	}
	if code, _ := do(t, "GET", base+"/", nil, "attacker.example:"+port); code != http.StatusForbidden {
		t.Errorf("foreign host on the page: %d", code)
	}
	if code, _ := do(t, "GET", base+"/api/app", ok, "localhost:"+port); code != http.StatusOK {
		t.Errorf("localhost: %d", code)
	}
	cross := map[string]string{tokenHeader: token, "Origin": "https://evil.example"}
	if code, _ := do(t, "POST", base+"/api/quit", cross, ""); code != http.StatusForbidden {
		t.Errorf("cross-origin POST: %d", code)
	}
	site := map[string]string{tokenHeader: token, "Sec-Fetch-Site": "cross-site"}
	if code, _ := do(t, "GET", base+"/api/app", site, ""); code != http.StatusForbidden {
		t.Errorf("cross-site fetch: %d", code)
	}
	same := map[string]string{tokenHeader: token, "Origin": base}
	if code, _ := do(t, "GET", base+"/api/app", same, ""); code != http.StatusOK {
		t.Errorf("same origin: %d", code)
	}
}

func TestPageHasAStrictContentSecurityPolicy(t *testing.T) {
	base := startServer(t)
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "connect-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q lacks %q", csp, want)
		}
	}
}

func TestQRCodeIsDrawnAsSVG(t *testing.T) {
	svg := qrSVG("2@abc,def,ghi")
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "<path") {
		t.Errorf("svg = %.80q", svg)
	}
	v := loginView(&whatsapp.LoginEvent{Kind: whatsapp.LoginQR, QR: "secret-code", ExpiresIn: 20 * time.Second})
	if v.QR != "" || v.QRSVG == "" || v.ExpiresIn != 20 {
		t.Errorf("view = %+v", v)
	}
}
