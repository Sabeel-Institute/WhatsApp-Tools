//go:build !unix && !windows

package platform

import "os"

// Systems without file locks fall back to "always acquired".
func tryLock(*os.File) (bool, error) { return true, nil }

func unlock(*os.File) error { return nil }
