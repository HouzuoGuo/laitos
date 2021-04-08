package slimerjs

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/platform"
)

func TestBrowserInstances(t *testing.T) {
	// CircleCI and WSL don't run containers
	platform.SkipIfWSL(t)
	platform.SkipTestIfCI(t)
	if os.Getuid() != 0 {
		t.Skip("this test involves docker daemon operation, it requires root privilege.")
	}

	instances := Instances{}
	if err := instances.Initialise(); !strings.Contains(err.Error(), "BasePortNumber") {
		t.Fatal(err)
	}
	instances.BasePortNumber = 30167
	// Test default settings
	if err := instances.Initialise(); err != nil || instances.MaxInstances != 5 || instances.MaxLifetimeSec != 1800 {
		t.Fatalf("%+v %+v", err, instances)
	}

	// Prepare settings for tests
	instances.MaxInstances = 2
	instances.MaxLifetimeSec = 300
	if err := instances.Initialise(); err != nil {
		t.Fatal(err)
	}
	defer instances.KillAll()

	// Prepare docker operation for SlimerJS
	PrepareDocker(lalog.Logger{})

	i0, b0, err := instances.Acquire()
	if i0 != 0 || b0.Tag == "" || err != nil {
		t.Fatal(i0, b0, err)
	}
	if platform.HostIsWindows() {
		// FIXME: can SlimerJS run more than one instance at a time on Windows? The second Acquire() fails.
		fmt.Println("FIXME: can SlimerJS run more than one instance at a time on Windows? The second Acquire() fails.")
		return
	}
	i1, b1, err := instances.Acquire()
	if i1 != 1 || b1.Tag == "" || err != nil {
		t.Fatal(i1, b1, err)
	}
	i2, b2, err := instances.Acquire()
	if i2 != 0 || b2.Tag == "" || err != nil {
		t.Fatal(i2, b2, err)
	}
	i3, b3, err := instances.Acquire()
	if i3 != 1 || b3.Tag == "" || err != nil {
		t.Fatal(i3, b3, err)
	}
	if b := instances.Retrieve(0, "wrong tag"); b != nil {
		t.Fatal("did not reject")
	}
	if b := instances.Retrieve(1, "wrong tag"); b != nil {
		t.Fatal("did not reject")
	}
	if b := instances.Retrieve(0, b2.Tag); b == nil {
		t.Fatal("did not retrieve")
	}
	if b := instances.Retrieve(1, b3.Tag); b == nil {
		t.Fatal("did not retrieve")
	}

	// Repeatedly stopping instance should have no negative consequence
	instances.KillAll()
	instances.KillAll()
	instances.KillAll()
}
