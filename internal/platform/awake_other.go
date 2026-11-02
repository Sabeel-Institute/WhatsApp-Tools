//go:build !linux && !darwin && !windows

package platform

import "errors"

func keepAwake(string) (func(), error) {
	return nil, errors.New("keeping the computer awake is not supported on this system")
}
