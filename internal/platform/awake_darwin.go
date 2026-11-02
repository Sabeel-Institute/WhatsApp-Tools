//go:build darwin

package platform

import (
	"os"
	"os/exec"
	"strconv"
)

// keepAwake runs caffeinate tied to this process: -i prevents idle sleep and
// -w exits it automatically if the app dies.
func keepAwake(string) (func(), error) {
	cmd := exec.Command("/usr/bin/caffeinate", "-i", "-w", strconv.Itoa(os.Getpid()))
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
