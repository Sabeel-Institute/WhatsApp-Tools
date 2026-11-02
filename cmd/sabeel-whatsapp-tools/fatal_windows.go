//go:build windows

package main

import (
	"golang.org/x/sys/windows"
)

// reportFatal shows a message box, because a -H=windowsgui build has no
// console where the error could appear.
func reportFatal(err error) {
	title, _ := windows.UTF16PtrFromString("Sabeel WhatsApp Tools")
	text, _ := windows.UTF16PtrFromString("The app couldn't start:\n\n" + err.Error())
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}
