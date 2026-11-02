//go:build windows

package platform

import (
	"os/exec"
	"syscall"
)

func openBrowser(url string) error {
	// rundll32 opens the default browser without flashing a console window,
	// which "cmd /c start" would do in a -H=windowsgui build.
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
