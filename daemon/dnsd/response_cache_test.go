package dnsd

import (
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/tcpoverdns"
)

func TestResponseCache(t *testing.T) {
	counter := 0
	setFun := func() []tcpoverdns.Segment {
		counter++
		return []tcpoverdns.Segment{{Data: []byte{byte(counter)}}}
	}
	cache := NewResponseCache(500*time.Millisecond, 10)
	for i := 0; i < 3; i++ {
		if got := cache.GetOrSet("a", setFun); len(got) != 1 || !got[0].Equals(tcpoverdns.Segment{Data: []byte{1}}) {
			t.Fatalf("i: %v, got: %v, want 1", i, got)
		}
	}
	time.Sleep(500 * time.Millisecond)
	for i := 0; i < 3; i++ {
		if got := cache.GetOrSet("a", setFun); len(got) != 1 || !got[0].Equals(tcpoverdns.Segment{Data: []byte{2}}) {
			t.Fatalf("i: %v, got: %v, want 2", i, got)
		}
	}
	for i := 0; i < 3; i++ {
		if got := cache.GetOrSet("b", setFun); len(got) != 1 || !got[0].Equals(tcpoverdns.Segment{Data: []byte{3}}) {
			t.Fatalf("i: %v, got: %v, want 3", i, got)
		}
	}
	if len(cache.cache) != 2 {
		t.Fatalf("unexpected cache items: %+v", cache.cache)
	}
}
