// Command sabeel-whatsapp-tools is the desktop app: it starts a private
// local web server and opens the user's browser to it.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/app"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/webui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		reportFatal(err)
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("sabeel-whatsapp-tools", flag.ContinueOnError)
	demo := fs.Bool("demo", false, "try the app with a pretend WhatsApp account (nothing is sent)")
	dataDir := fs.String("data-dir", "", "keep the app's data in this folder instead of the default")
	port := fs.Int("port", 0, "local port for the user interface (default: any free port)")
	noBrowser := fs.Bool("no-browser", false, "don't open the browser; print the address instead")
	verbose := fs.Bool("verbose", false, "also write the log to the terminal")
	showVersion := fs.Bool("version", false, "print the version and exit")
	selfTest := fs.Bool("self-test", false, "start, check the user interface responds, and exit (for CI)")
	devOrigin := fs.String("dev-origin", "", "developers: allow requests from this origin (the Vite dev server)")
	devToken := fs.String("dev-token", "", "developers: use this session token instead of a random one")
	demoLatency := fs.Float64("demo-latency", 1, "demo mode: multiply simulated delays by this factor")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Println(app.Name, version)
		return nil
	}
	if *port < 0 || *port > 65535 {
		return fmt.Errorf("--port must be between 0 and 65535, got %d", *port)
	}
	if *demoLatency <= 0 || *demoLatency > 10 {
		return fmt.Errorf("--demo-latency must be greater than 0 and at most 10, got %v", *demoLatency)
	}
	if *devOrigin != "" {
		u, err := url.Parse(*devOrigin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" {
			return fmt.Errorf("--dev-origin must look like http://localhost:5173, got %q", *devOrigin)
		}
	}
	if *devToken != "" && len(*devToken) < 8 {
		return fmt.Errorf("--dev-token must be at least 8 characters")
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q (see --help)", fs.Arg(0))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := app.Options{
		Demo:        *demo,
		DataDir:     *dataDir,
		Port:        *port,
		NoBrowser:   *noBrowser || *selfTest,
		DevOrigin:   *devOrigin,
		Token:       *devToken,
		Version:     version,
		Verbose:     *verbose,
		Static:      webui.FS(),
		DemoLatency: *demoLatency,
	}
	if *noBrowser && !*selfTest {
		opts.OnReady = func(u string) { fmt.Println("Open this address in your browser:", u) }
	}
	if *selfTest {
		return runSelfTest(ctx, opts)
	}
	return app.Run(ctx, opts)
}

// runSelfTest starts the app in a temporary folder, fetches the page and the
// API with the session token, and quits. CI runs it on every platform.
func runSelfTest(parent context.Context, opts app.Options) error {
	dir, err := os.MkdirTemp("", "sabeel-wa-selftest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	opts.DataDir = dir
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	result := make(chan error, 1)
	opts.OnReady = func(appURL string) {
		go func() {
			result <- checkServing(appURL)
			cancel()
		}()
	}
	if err := app.Run(ctx, opts); err != nil {
		return err
	}
	select {
	case err := <-result:
		if err == nil {
			fmt.Println("self-test passed")
		}
		return err
	default:
		return errors.New("self-test timed out")
	}
}

func checkServing(appURL string) error {
	u, err := url.Parse(appURL)
	if err != nil {
		return err
	}
	base := "http://" + u.Host
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(base + "/")
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("page returned %s", resp.Status)
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/api/app", nil)
	req.Header.Set("X-App-Token", strings.TrimPrefix(u.Fragment, "token="))
	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api returned %s", resp.Status)
	}
	unauth, _ := http.NewRequest(http.MethodGet, base+"/api/app", nil)
	resp, err = client.Do(unauth)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("api without token returned %s, want 401", resp.Status)
	}
	return nil
}
