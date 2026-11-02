package whatsapp

import "sync"

// Broadcaster fans events out to subscribers. Slow subscribers lose events
// rather than blocking WhatsApp's event loop; every event carries a full
// snapshot, so the next one corrects them.
type Broadcaster struct {
	mu   sync.Mutex
	next int
	subs map[int]chan Event
}

// Subscribe registers a subscriber.
func (b *Broadcaster) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs == nil {
		b.subs = make(map[int]chan Event)
	}
	id := b.next
	b.next++
	ch := make(chan Event, 32)
	b.subs[id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if c, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(c)
		}
	}
}

// Publish delivers e to every subscriber without blocking.
func (b *Broadcaster) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
