//go:build darwin

package platform

import "os/exec"

func openBrowser(url string) error {
	cmd := exec.Command("open", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
