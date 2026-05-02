package acpbridge

import "sync"

type StampedFrame struct {
	ID    uint64
	Frame []byte
}

type Ring struct {
	mu     sync.Mutex
	cap    int
	buf    []StampedFrame
	nextID uint64
}

func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		capacity = 500
	}
	return &Ring{cap: capacity, buf: make([]StampedFrame, 0, capacity)}
}

// Push stores a copy of frame and returns its assigned id.
func (r *Ring) Push(frame []byte) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	cp := make([]byte, len(frame))
	copy(cp, frame)
	sf := StampedFrame{ID: r.nextID, Frame: cp}
	if len(r.buf) < r.cap {
		r.buf = append(r.buf, sf)
	} else {
		copy(r.buf, r.buf[1:])
		r.buf[len(r.buf)-1] = sf
	}
	return r.nextID
}

// Since returns all stamped frames whose ID is > after, in order.
func (r *Ring) Since(after uint64) []StampedFrame {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]StampedFrame, 0, len(r.buf))
	for _, sf := range r.buf {
		if sf.ID > after {
			out = append(out, sf)
		}
	}
	return out
}
