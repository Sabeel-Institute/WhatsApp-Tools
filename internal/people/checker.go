package people

import (
	"sync"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// checkTTL is how long a "has WhatsApp" answer is trusted.
const checkTTL = 6 * time.Hour

// Checker looks numbers up on WhatsApp in paced batches and caches the
// answers, so review screens and runs share one source of truth.
type Checker struct {
	wa     whatsapp.Client
	pacing whatsapp.Pacing

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	check whatsapp.NumberCheck
	at    time.Time
}

// NewChecker returns a Checker.
func NewChecker(wa whatsapp.Client, pacing whatsapp.Pacing) *Checker {
	return &Checker{wa: wa, pacing: pacing, cache: map[string]cached{}}
}

// Lookup returns a cached answer.
func (c *Checker) Lookup(phone string) (onWhatsApp bool, known bool) {
	r, ok := c.Get(phone)
	return r.OnWhatsApp, ok
}

// Get returns the cached check for phone.
func (c *Checker) Get(phone string) (whatsapp.NumberCheck, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.cache[phone]
	if !ok || time.Since(e.at) > checkTTL {
		return whatsapp.NumberCheck{}, false
	}
	return e.check, true
}

// Unchecked filters phones down to those without a cached answer,
// preserving order and dropping repeats.
func (c *Checker) Unchecked(phones []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range phones {
		if seen[p] {
			continue
		}
		seen[p] = true
		if _, ok := c.Get(p); !ok {
			out = append(out, p)
		}
	}
	return out
}

// Forget clears the cache (used after unlinking).
func (c *Checker) Forget() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = map[string]cached{}
}

// Check looks up phones not already cached, in paced batches, reporting
// progress through ctl. progress is called after each batch with the
// number of phones checked so far.
func (c *Checker) Check(ctl *jobs.Control, phones []string, progress func(done int)) error {
	todo := c.Unchecked(phones)
	size := max(c.pacing.CheckBatchSize, 1)
	done := 0
	for start := 0; start < len(todo); start += size {
		if err := ctl.Checkpoint(); err != nil {
			return err
		}
		batch := todo[start:min(start+size, len(todo))]
		results, err := Call(ctl, c.wa, c.pacing, func() ([]whatsapp.NumberCheck, error) {
			ctx, cancel := ctl.Context()
			defer cancel()
			return c.wa.CheckNumbers(ctx, batch)
		})
		if err != nil {
			return err
		}
		now := time.Now()
		c.mu.Lock()
		for _, r := range results {
			c.cache[r.Phone] = cached{check: r, at: now}
		}
		c.mu.Unlock()
		done += len(batch)
		if progress != nil {
			progress(done)
		}
		if start+size < len(todo) {
			if err := ctl.Sleep(whatsapp.Between(c.pacing.CheckPauseMin, c.pacing.CheckPauseMax)); err != nil {
				return err
			}
		}
	}
	return nil
}
