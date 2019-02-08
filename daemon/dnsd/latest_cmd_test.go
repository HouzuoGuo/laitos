package dnsd

import (
	"github.com/HouzuoGuo/laitos/toolbox"
	"testing"
	"time"
)

func TestLatestCommands(t *testing.T) {
	rec := NewLatestCommands()
	if r := rec.Get("does-not-exist"); r != nil {
		t.Fatal(r)
	}
	rec.StoreResult("input", &toolbox.Result{CombinedOutput: "output"})
	if r := rec.Get("input"); r == nil || r.CombinedOutput != "output" {
		t.Fatal(r)
	}
	time.Sleep((TextCommandReplyTTL + 1) * time.Second)
	if len(rec.latestResult) != 0 {
		t.Fatal(rec.latestResult)
	}
	if r := rec.Get("input"); r != nil {
		t.Fatal(r)
	}
	rec.StoreResult("input", &toolbox.Result{CombinedOutput: "another-output"})
	if r := rec.Get("input"); r == nil || r.CombinedOutput != "another-output" {
		t.Fatal(r)
	}
}
