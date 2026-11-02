//go:build linux

package platform

import (
	"os/exec"
)

// keepAwake holds a systemd-logind inhibitor lock for as long as a child
// "sleep" process runs. Desktops without systemd simply return an error.
func keepAwake(reason string) (func(), error) {
	path, err := exec.LookPath("systemd-inhibit")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, "--what=sleep:idle", "--who=Sabeel WhatsApp Tools",
		"--why="+reason, "--mode=block", "sleep", "infinity")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	return func() {
		_ = cmd.Process.Kill()
		<-done
	}, nil
}
