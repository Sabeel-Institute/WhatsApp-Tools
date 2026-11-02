//go:build windows

package platform

import (
	"runtime"

	"golang.org/x/sys/windows"
)

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var setThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// keepAwake calls SetThreadExecutionState from one locked OS thread, because
// the request belongs to the thread that made it.
func keepAwake(string) (func(), error) {
	if err := setThreadExecutionState.Find(); err != nil {
		return nil, err
	}
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		_, _, _ = setThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired))
		<-release
		_, _, _ = setThreadExecutionState.Call(uintptr(esContinuous))
		close(done)
	}()
	return func() {
		close(release)
		<-done
	}, nil
}
