package launcher

import (
	"reflect"
	"strings"
	"testing"
)

func TestRemoveFromFlags(t *testing.T) {
	// Remove flag and value
	flags := []string{"/a/b/c", "-aaa", "123", "-bbb", "ccc", "-ddd"}
	ret := removeFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-a")
	}, flags)
	if !reflect.DeepEqual(ret, []string{"/a/b/c", "-bbb", "ccc", "-ddd"}) {
		t.Fatal(ret)
	}

	// Remove flag=value
	flags = []string{"/a/b/c", "-aaa", "-bbb=123", "ccc", "-ddd"}
	ret = removeFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-b")
	}, flags)
	if !reflect.DeepEqual(ret, []string{"/a/b/c", "-aaa", "ccc", "-ddd"}) {
		t.Fatal(ret)
	}

	// Remove non-existent flag
	ret = removeFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-doesnotexist")
	}, flags)
	if !reflect.DeepEqual(ret, []string{"/a/b/c", "-aaa", "-bbb=123", "ccc", "-ddd"}) {
		t.Fatal(ret)
	}
}

func TestSupervisor_GetLaunchParameters(t *testing.T) {
	sup := &Supervisor{
		CLIArgs:      []string{"/a", "-disableconflicts", "-tunesystem", "-swapoff", "-gomaxprocs", "16", "-config", "config.json", "-daemons", "dnsd,httpd,insecurehttpd,maintenance,plainsocket,smtpd,sockd,telegram"},
		Config:       Config{},
		DaemonNames:  nil,
		ShedSequence: nil,
	}
	sup.makeShedSequence()
}
