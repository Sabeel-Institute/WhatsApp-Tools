//go:build !windows

package main

// reportFatal is a no-op where the app is started from a terminal or a
// launcher that shows stderr.
func reportFatal(error) {}
