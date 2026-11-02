package platform

import "sync"

// KeepAwake asks the OS not to sleep while a job runs. The returned function
// releases the request; calling it more than once is harmless. Failure to
// inhibit sleep is not an error worth surfacing: the job still runs, and
// the caller logs the returned error.
func KeepAwake(reason string) (release func(), err error) {
	stop, err := keepAwake(reason)
	if err != nil {
		return func() {}, err
	}
	var once sync.Once
	return func() { once.Do(stop) }, nil
}
