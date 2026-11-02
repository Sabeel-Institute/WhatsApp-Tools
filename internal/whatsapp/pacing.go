package whatsapp

import (
	"math/rand/v2"
	"time"
)

// Pacing controls how fast the app talks to WhatsApp. WhatsApp restricts
// accounts that add or message many people quickly, so every value errs on
// the slow side. This is the one place to tune it.
type Pacing struct {
	// AddBatchSize people are added per request.
	AddBatchSize int
	// A random pause in [BatchPauseMin, BatchPauseMax] follows each batch.
	BatchPauseMin, BatchPauseMax time.Duration

	// CheckBatchSize numbers are looked up per request.
	CheckBatchSize int
	CheckPauseMin  time.Duration
	CheckPauseMax  time.Duration

	// A random pause in [InvitePauseMin, InvitePauseMax] follows each
	// personal invite message.
	InvitePauseMin, InvitePauseMax time.Duration

	// Retries of temporary failures wait Backoff[i] (with ±25% jitter).
	// The length of Backoff is the retry limit.
	Backoff []time.Duration
	// When WhatsApp says "slow down", wait this long, doubling each time.
	RateLimitPause time.Duration
	// Give up after this many consecutive "slow down" answers.
	MaxRateLimitWaits int
}

// DefaultPacing is used with a real WhatsApp account.
var DefaultPacing = Pacing{
	AddBatchSize:      5,
	BatchPauseMin:     4 * time.Second,
	BatchPauseMax:     9 * time.Second,
	CheckBatchSize:    20,
	CheckPauseMin:     2 * time.Second,
	CheckPauseMax:     4 * time.Second,
	InvitePauseMin:    8 * time.Second,
	InvitePauseMax:    15 * time.Second,
	Backoff:           []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second},
	RateLimitPause:    60 * time.Second,
	MaxRateLimitWaits: 4,
}

// Scaled returns a copy with every delay multiplied by f. Demo mode uses it
// so trying the app does not take as long as a real run.
func (p Pacing) Scaled(f float64) Pacing {
	s := func(d time.Duration) time.Duration { return time.Duration(float64(d) * f) }
	out := p
	out.BatchPauseMin, out.BatchPauseMax = s(p.BatchPauseMin), s(p.BatchPauseMax)
	out.CheckPauseMin, out.CheckPauseMax = s(p.CheckPauseMin), s(p.CheckPauseMax)
	out.InvitePauseMin, out.InvitePauseMax = s(p.InvitePauseMin), s(p.InvitePauseMax)
	out.RateLimitPause = s(p.RateLimitPause)
	out.Backoff = make([]time.Duration, len(p.Backoff))
	for i, d := range p.Backoff {
		out.Backoff[i] = s(d)
	}
	return out
}

// Between returns a uniformly random duration in [lo, hi].
func Between(lo, hi time.Duration) time.Duration {
	if hi <= lo {
		return lo
	}
	return lo + time.Duration(rand.Int64N(int64(hi-lo)+1))
}

// Jitter returns d ±25%.
func Jitter(d time.Duration) time.Duration {
	return Between(d*3/4, d*5/4)
}
