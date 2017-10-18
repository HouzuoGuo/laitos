package launcher

import (
	"reflect"
	"strings"
	"testing"
)

func TestRemoveFromFlags(t *testing.T) {
	// Remove flag and value in middle
	flags := []string{"-aaa", "123", "-bbb", "ccc", "-ddd"}
	ret := RemoveFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-a")
	}, flags)
	if !reflect.DeepEqual(ret, []string{"-bbb", "ccc", "-ddd"}) {
		t.Fatal(ret)
	}

	// Remove flag and value at end
	flags = []string{"-aaa", "123", "-bbb", "-ccc", "ddd"}
	ret = RemoveFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-c")
	}, flags)
	if !reflect.DeepEqual(ret, []string{"-aaa", "123", "-bbb"}) {
		t.Fatal(ret)
	}

	// Remove flag=value in middle
	flags = []string{"-aaa", "-bbb=123", "ccc", "-ddd"}
	ret = RemoveFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-b")
	}, flags)
	if !reflect.DeepEqual(ret, []string{"-aaa", "ccc", "-ddd"}) {
		t.Fatal(ret)
	}

	// Remove flag=value at end
	flags = []string{"-aaa", "-bbb", "-ccc", "-ddd=123"}
	ret = RemoveFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-d")
	}, flags)
	if !reflect.DeepEqual(ret, []string{"-aaa", "-bbb", "-ccc"}) {
		t.Fatal(ret)
	}

	// Remove non-existent flag
	flags = []string{"-aaa", "-bbb", "-ccc", "-ddd=123"}
	ret = RemoveFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-doesnotexist")
	}, flags)
	if !reflect.DeepEqual(ret, []string{"-aaa", "-bbb", "-ccc", "-ddd=123"}) {
		t.Fatal(ret)
	}
}

func TestSupervisor_GetLaunchParameters(t *testing.T) {
	originalCLIFlags := []string{"-disableconflicts", "-tunesystem", "-swapoff", "-gomaxprocs", "16", "-config", "config.json", "-daemons", "httpd,maintenance,smtpd,telegram"}
	originalDaemonList := []string{"httpd", "maintenance", "smtpd", "telegram"}
	sup := &Supervisor{CLIFlags: originalCLIFlags, DaemonNames: originalDaemonList}
	sup.initialise()

	// Verify daemon shedding sequence
	shedSequenceMatch := [][]string{
		{"httpd", "smtpd", "telegram"},
		{"httpd", "telegram"},
		{"telegram"},
	}
	if !reflect.DeepEqual(shedSequenceMatch, sup.shedSequence) {
		t.Fatal(sup.shedSequence)
	}

	// The first (0th) attempt should launch main program pretty much same set of parameters
	flags, daemons := sup.GetLaunchParameters(0)
	if !reflect.DeepEqual(flags, []string{"-disableconflicts", "-tunesystem", "-swapoff", "-gomaxprocs", "16", "-config", "config.json", "-supervisor=false", "-daemons", "httpd,maintenance,smtpd,telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, originalDaemonList) {
		t.Fatal(daemons)
	}

	// The second attempt should launch main program using reduced set of parameters, but same set of daemons.
	flags, daemons = sup.GetLaunchParameters(1)
	if !reflect.DeepEqual(flags, []string{"-config", "config.json", "-supervisor=false", "-daemons", "httpd,maintenance,smtpd,telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, originalDaemonList) {
		t.Fatal(daemons)
	}

	// The third attempt should shed maintenance daemon
	flags, daemons = sup.GetLaunchParameters(2)
	if !reflect.DeepEqual(flags, []string{"-config", "config.json", "-supervisor=false", "-daemons", "httpd,smtpd,telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, []string{"httpd", "smtpd", "telegram"}) {
		t.Fatal(daemons)
	}

	// The fourth attempt should shed SMTP daemon
	flags, daemons = sup.GetLaunchParameters(3)
	if !reflect.DeepEqual(flags, []string{"-config", "config.json", "-supervisor=false", "-daemons", "httpd,telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, []string{"httpd", "telegram"}) {
		t.Fatal(daemons)
	}

	// The fifth attempt should shed HTTP daemon
	flags, daemons = sup.GetLaunchParameters(4)
	if !reflect.DeepEqual(flags, []string{"-config", "config.json", "-supervisor=false", "-daemons", "telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, []string{"telegram"}) {
		t.Fatal(daemons)
	}

	// All further attempts should not shed any daemons, but only remove non-essential flags.
	for i := 5; i < 500; i++ {
		flags, daemons = sup.GetLaunchParameters(5)
		if !reflect.DeepEqual(flags, []string{"-config", "config.json", "-supervisor=false", "-daemons", "httpd,maintenance,smtpd,telegram"}) {
			t.Fatal(flags)
		}
		if !reflect.DeepEqual(daemons, originalDaemonList) {
			t.Fatal(daemons)
		}
	}
}
