package store

import "sync"

// Ring is a thread-safe fixed-capacity ring buffer for recent spans.
type Ring struct {
	mu   sync.RWMutex
	buf  []SpanRecord
	cap  int
	head int // next write position
	len  int
}

// NewRing creates a ring buffer with the given capacity.
func NewRing(capacity int) *Ring {
	return &Ring{
		buf: make([]SpanRecord, capacity),
		cap: capacity,
	}
}

// Push adds a span to the ring buffer, overwriting the oldest if full.
func (r *Ring) Push(span SpanRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = span
	r.head = (r.head + 1) % r.cap
	if r.len < r.cap {
		r.len++
	}
}

// PushBatch adds multiple spans to the ring buffer.
func (r *Ring) PushBatch(spans []SpanRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range spans {
		r.buf[r.head] = spans[i]
		r.head = (r.head + 1) % r.cap
		if r.len < r.cap {
			r.len++
		}
	}
}

// Recent returns the most recent n spans in reverse chronological order.
func (r *Ring) Recent(n int) []SpanRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if n <= 0 || r.len == 0 {
		return nil
	}
	if n > r.len {
		n = r.len
	}

	result := make([]SpanRecord, n)
	pos := (r.head - 1 + r.cap) % r.cap
	for i := 0; i < n; i++ {
		result[i] = r.buf[pos]
		pos = (pos - 1 + r.cap) % r.cap
	}
	return result
}

// Len returns the number of spans currently in the ring buffer.
func (r *Ring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.len
}
