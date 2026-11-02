// Package app wires everything together and owns the desktop lifecycle:
// single instance, opening the browser, idle shutdown and Quit.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/logging"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/people"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/platform"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/runs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/server"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/settings"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp/demo"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp/real"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/workflows"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/workflows/coursegroups"
)

// Name is the product name shown everywhere.
const Name = "Sabeel WhatsApp Tools"

// IdleTimeout is how long the app waits for the browser before quitting by
// itself, when no job is running.
const IdleTimeout = 15 * time.Minute

// Options come from the command line.
type Options struct {
	Demo        bool
	DataDir     string // overrides the default location
	Port        int    // 0 picks a free port
	NoBrowser   bool
	DevOrigin   string // allow a development server origin
	Token       string // fixed token for development; random when empty
	Version     string
	Verbose     bool             // also log to the terminal
	Static      fs.FS            // the built web UI
	OnReady     func(url string) // called once serving (used by --self-test)
	DemoLatency float64          // demo mode speed (1 = realistic)
}

// Run starts the app and blocks until it quits.
func Run(ctx context.Context, opts Options) error {
	dir, err := platform.DataDir(opts.DataDir)
	if err != nil {
		return err
	}
	if opts.Demo {
		// Demo mode keeps its own history and settings.
		dir = filepath.Join(dir, "demo")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}

	lock, err := platform.AcquireInstanceLock(dir)
	if errors.Is(err, platform.ErrAlreadyRunning) {
		return focusRunningInstance(dir, opts)
	} else if err != nil {
		return err
	}
	defer lock.Release()

	logFile, err := logging.OpenRotatingFile(dir)
	if err != nil {
		return fmt.Errorf("opening log: %w", err)
	}
	defer logFile.Close()
	level := slog.LevelInfo
	log := logging.New(logFile, opts.Verbose, level)
	log.Info("starting", "version", opts.Version, "demo", opts.Demo)

	settingsStore, err := settings.Open(dir)
	if err != nil {
		return fmt.Errorf("opening settings: %w", err)
	}
	runStore, err := runs.Open(dir)
	if err != nil {
		return fmt.Errorf("opening history: %w", err)
	}

	var wa whatsapp.Client
	pacing := whatsapp.DefaultPacing
	if opts.Demo {
		latency := opts.DemoLatency
		if latency == 0 {
			latency = 1
		}
		wa = demo.New(dir, demo.Options{Latency: latency})
		pacing = pacing.Scaled(0.25 * latency)
	} else {
		wa = real.New(dir, log, versionTriple(opts.Version))
	}
	startCtx, cancelStart := context.WithTimeout(ctx, 30*time.Second)
	err = wa.Start(startCtx)
	cancelStart()
	if err != nil {
		return fmt.Errorf("starting WhatsApp: %w", err)
	}
	defer wa.Close()

	jobManager := jobs.NewManager()
	var awakeMu sync.Mutex
	release := func() {}
	jobManager.OnBusy = func(busy bool) {
		awakeMu.Lock()
		defer awakeMu.Unlock()
		release()
		release = func() {}
		if busy {
			r, err := platform.KeepAwake("Adding people to a WhatsApp group")
			if err != nil {
				log.Info("could not prevent sleep during the job", "err", err)
			}
			release = r
		}
	}
	defer func() { awakeMu.Lock(); release(); awakeMu.Unlock() }()

	deps := workflows.Deps{
		WA:       wa,
		Jobs:     jobManager,
		Runs:     runStore,
		Settings: settingsStore,
		Checker:  people.NewChecker(wa, pacing),
		Pacing:   pacing,
		Log:      log,
	}
	wfs := []workflows.Workflow{coursegroups.New(deps)}

	token := opts.Token
	if token == "" {
		token = randomToken()
	}
	quit := make(chan struct{})
	var quitOnce sync.Once
	var devOrigins []string
	if opts.DevOrigin != "" {
		devOrigins = append(devOrigins, opts.DevOrigin)
	}
	srv := server.New(server.Config{
		AppName:    Name,
		Version:    opts.Version,
		Demo:       opts.Demo,
		Token:      token,
		DataDir:    dir,
		DevOrigins: devOrigins,
		Static:     opts.Static,
		Log:        log,
		Quit:       func() { quitOnce.Do(func() { close(quit) }) },
	}, deps, wfs)

	ln, err := srv.Listen(opts.Port)
	if err != nil {
		return fmt.Errorf("starting the local server: %w", err)
	}
	appURL := fmt.Sprintf("http://127.0.0.1:%d/#token=%s", srv.Port(), token)
	if err := lock.Publish(platform.RunningInstance{URL: appURL, PID: os.Getpid()}); err != nil {
		log.Warn("could not record the running instance", "err", err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	log.Info("serving", "port", srv.Port())

	if opts.OnReady != nil {
		opts.OnReady(appURL)
	}
	if !opts.NoBrowser {
		if err := platform.OpenBrowser(appURL); err != nil {
			log.Warn("could not open the browser", "err", err)
			fmt.Fprintln(os.Stderr, "Open this address in your browser:", appURL)
		}
	}

	idle := time.NewTicker(time.Minute)
	defer idle.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-quit:
			log.Info("quit requested")
			break loop
		case err := <-serveErr:
			return err
		case <-idle.C:
			if srv.IdleFor() > IdleTimeout && !jobManager.Busy() {
				log.Info("no browser activity; shutting down")
				break loop
			}
		}
	}

	// Never stop in the middle of a job: wait for it (Ctrl+C from a
	// terminal is the only way here while one runs).
	for jobManager.Busy() {
		if snap, ok := jobManager.Active(); ok {
			jobManager.Stop(snap.ID)
		}
		time.Sleep(200 * time.Millisecond)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("stopped")
	return nil
}

// focusRunningInstance opens the browser on the copy that is already running.
func focusRunningInstance(dir string, opts Options) error {
	var info platform.RunningInstance
	var err error
	for i := 0; i < 20; i++ { // the other copy may still be starting
		if info, err = platform.ReadRunningInstance(dir); err == nil && ping(info.URL) {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("the app is already running but could not be reached: %w", err)
	}
	if opts.NoBrowser {
		fmt.Println("Already running at", info.URL)
		return nil
	}
	return platform.OpenBrowser(info.URL)
}

func ping(appURL string) bool {
	u, err := url.Parse(appURL)
	if err != nil {
		return false
	}
	token := strings.TrimPrefix(u.Fragment, "token=")
	req, err := http.NewRequest(http.MethodGet, "http://"+u.Host+"/api/app", nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-App-Token", token)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// versionTriple turns "1.2.3" (or "v1.2.3-4-gabc") into {1,2,3} for the
// linked-device label.
func versionTriple(v string) [3]uint32 {
	var out [3]uint32
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	for i, part := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(part)
		out[i] = uint32(max(n, 0))
	}
	return out
}
