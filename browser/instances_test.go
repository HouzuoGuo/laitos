package browser

import (
	"github.com/HouzuoGuo/laitos/misc"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
)

func TestBrowserInstances(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Because the built-in PhantomJS executable only works in linux/amd64, your system cannot run this test.")
	}
	// CircleCI container does not have the dependencies for running PhantomJS
	misc.SkipTestIfCI(t)
	instances := Instances{}
	if err := instances.Initialise(); !strings.Contains(err.Error(), "cannot find PhantomJS") {
		t.Fatal(err)
	}
	instances.PhantomJSExecPath = path.Join(os.Getenv("GOPATH"), "/src/github.com/HouzuoGuo/laitos/extra/phantomjs-2.1.1-linux-x86_64")
	if err := instances.Initialise(); !strings.Contains(err.Error(), "BasePortNumber") {
		t.Fatal(err)
	}
	instances.BasePortNumber = 65110
	// Test default settings
	if err := instances.Initialise(); err != nil || instances.MaxInstances != 5 || instances.MaxLifetimeSec != 1800 {
		t.Fatalf("%+v %+v", err, instances)
	}

	// Prepare settings for tests
	instances.MaxInstances = 2
	instances.MaxLifetimeSec = 60
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
