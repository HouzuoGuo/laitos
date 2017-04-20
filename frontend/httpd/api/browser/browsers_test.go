package browser

import (
	"os"
	"path"
	"strings"
	"testing"
)

func TestBrowserInstances(t *testing.T) {
	instances := Browsers{}
	if err := instances.Initialise(); !strings.Contains(err.Error(), "cannot find PhantomJS") {
		t.Fatal(err)
	}
	instances.PhantomJSExecPath = path.Join(os.Getenv("GOPATH"), "/src/github.com/HouzuoGuo/laitos/addon/phantomjs-2.1.1-linux-x86_64")
	if err := instances.Initialise(); !strings.Contains(err.Error(), "MaxInstances") {
		t.Fatal(err)
	}
	instances.MaxInstances = 2
	if err := instances.Initialise(); !strings.Contains(err.Error(), "MaxLifetimeSec") {
		t.Fatal(err)
	}
	instances.MaxLifetimeSec = 60
	if err := instances.Initialise(); !strings.Contains(err.Error(), "BasePortNumber") {
		t.Fatal(err)
	}
	instances.BasePortNumber = 65110
	if err := instances.Initialise(); err != nil {
		t.Fatal(err)
	}
	defer instances.StopAll()

	i0, b0, err := instances.Acquire()
	if i0 != 0 || b0.Tag != "1" || err != nil {
		t.Fatal(i0, b0, err)
	}
	i1, b1, err := instances.Acquire()
	if i1 != 1 || b1.Tag != "2" || err != nil {
		t.Fatal(i1, b1, err)
	}
	i2, b2, err := instances.Acquire()
	if i2 != 0 || b2.Tag != "3" || err != nil {
		t.Fatal(i2, b2, err)
	}
	i3, b3, err := instances.Acquire()
	if i3 != 1 || b3.Tag != "4" || err != nil {
		t.Fatal(i3, b3, err)
	}
	if b := instances.Retrieve(0, "1"); b != nil {
		t.Fatal("did not reject")
	}
	if b := instances.Retrieve(1, "2"); b != nil {
		t.Fatal("did not reject")
	}
	if b := instances.Retrieve(0, "3"); b == nil {
		t.Fatal("did not retrieve")
	}
	if b := instances.Retrieve(1, "4"); b == nil {
		t.Fatal("did not retrieve")
	}
}
