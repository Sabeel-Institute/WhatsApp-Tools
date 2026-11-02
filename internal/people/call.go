package people

import (
	"strconv"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/api"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// Call runs one WhatsApp request inside a job, handling the failures every
// workflow meets the same way:
//
//   - offline: wait (shown as "waiting") until WhatsApp is connected again,
//     then retry without using up an attempt;
//   - "slow down": wait RateLimitPause, doubling, up to MaxRateLimitWaits;
//   - temporary errors: retry after each Backoff delay;
//   - anything else: return a *jobs.Failure with a plain-language message.
//
// Pause and stop requests are honoured during every wait.
func Call[T any](ctl *jobs.Control, wa whatsapp.Client, p whatsapp.Pacing, op func() (T, error)) (T, error) {
	attempts, rateWaits := 0, 0
	for {
		v, err := op()
		if err == nil {
			ctl.Waiting("")
			return v, nil
		}
		var zero T
		kind := whatsapp.KindOf(err)
		switch {
		case kind == whatsapp.KindNotConnected:
			if werr := waitForConnection(ctl, wa); werr != nil {
				return zero, werr
			}
			continue
		case kind == whatsapp.KindRateLimited && rateWaits < p.MaxRateLimitWaits:
			wait := p.RateLimitPause << rateWaits
			rateWaits++
			ctl.Waiting("WhatsApp asked us to slow down. Continuing in " + roundDuration(wait) + ".")
			if serr := ctl.Sleep(wait); serr != nil {
				return zero, serr
			}
			continue
		case kind == whatsapp.KindTemporary && attempts < len(p.Backoff):
			wait := whatsapp.Jitter(p.Backoff[attempts])
			attempts++
			ctl.Waiting("WhatsApp didn't respond. Trying again in a few seconds…")
			if serr := ctl.Sleep(wait); serr != nil {
				return zero, serr
			}
			continue
		}
		ctl.Waiting("")
		return zero, &jobs.Failure{Message: api.WhatsAppMessage(kind), Err: err}
	}
}

// waitForConnection blocks until WhatsApp reports connected, failing if the
// account was unlinked meanwhile.
func waitForConnection(ctl *jobs.Control, wa whatsapp.Client) error {
	ctl.Waiting("Waiting for the internet connection to come back. The app continues by itself.")
	for {
		switch wa.Status().State {
		case whatsapp.StateConnected:
			// Give the fresh connection a moment to settle before resuming.
			return ctl.Sleep(2 * time.Second)
		case whatsapp.StateLoggedOut, whatsapp.StateUnlinked:
			return &jobs.Failure{Message: "WhatsApp disconnected this computer, so the app stopped. The results so far are saved. Connect again to continue."}
		case whatsapp.StateBanned:
			return &jobs.Failure{Message: "WhatsApp has temporarily restricted this account, so the app stopped. The results so far are saved."}
		case whatsapp.StateOutdated:
			return &jobs.Failure{Message: "WhatsApp needs a newer version of this app, so it stopped. The results so far are saved."}
		case whatsapp.StateReplaced:
			return &jobs.Failure{Message: "This WhatsApp link was opened somewhere else, so the app stopped. The results so far are saved."}
		}
		if err := ctl.Sleep(time.Second); err != nil {
			return err
		}
	}
}

func roundDuration(d time.Duration) string {
	if d >= 90*time.Second {
		return strconv.Itoa(int(d.Minutes()+0.5)) + " minutes"
	}
	return strconv.Itoa(int(d.Seconds()+0.5)) + " seconds"
}
