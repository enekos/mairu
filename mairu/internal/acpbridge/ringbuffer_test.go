package acpbridge

import (
	"reflect"
	"testing"
)

func TestRingPushAndSinceEmpty(t *testing.T) {
	r := NewRing(3)
	if got := r.Since(0); len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}

func TestRingPushAssignsMonotonicIDs(t *testing.T) {
	r := NewRing(4)
	a := r.Push([]byte("a"))
	b := r.Push([]byte("b"))
	c := r.Push([]byte("c"))
	if a != 1 || b != 2 || c != 3 {
		t.Fatalf("ids = %d,%d,%d, want 1,2,3", a, b, c)
	}
}

func TestRingSinceReturnsNewer(t *testing.T) {
	r := NewRing(4)
	r.Push([]byte("a"))
	r.Push([]byte("b"))
	r.Push([]byte("c"))
	got := r.Since(1)
	if len(got) != 2 || string(got[0].Frame) != "b" || got[0].ID != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestRingEvictsOldest(t *testing.T) {
	r := NewRing(2)
	r.Push([]byte("a")) // id=1, evicted
	r.Push([]byte("b")) // id=2
	r.Push([]byte("c")) // id=3
	got := r.Since(0)
	ids := []uint64{got[0].ID, got[1].ID}
	if !reflect.DeepEqual(ids, []uint64{2, 3}) {
		t.Fatalf("ids = %v want [2 3]", ids)
	}
}

func TestRingSinceAfterEvictionReturnsAvailable(t *testing.T) {
	r := NewRing(2)
	r.Push([]byte("a"))
	r.Push([]byte("b"))
	r.Push([]byte("c")) // evicts a
	got := r.Since(1)   // client wants id>1, but id=2 still present
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
}
