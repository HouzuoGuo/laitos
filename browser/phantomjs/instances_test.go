package phantomjs

import (
	"runtime"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

func TestBrowserInstances(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Because the built-in PhantomJS executable only works in linux/amd64, your system cannot run this test.")
	}
	// Preparation copies PhantomJS executable into a utilities directory and adds it to program $PATH.
	misc.CopyNonEssentialUtilities(lalog.Logger{})
	// CircleCI container does not have the dependencies for running PhantomJS
	misc.SkipTestIfCI(t)
	instances := Instances{}
	if err := instances.Initialise(); !strings.Contains(err.Error(), "BasePortNumber") {
		t.Fatal(err)
	}
	instances.BasePortNumber = 65110
	instances.PhantomJSExecPath = "" // automatically find phantomjs among $PATH
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

	i0, b0, err := instances.Acquire()
	if i0 != 0 || b0.Tag == "" || err != nil {
		t.Fatal(i0, b0, err)
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
