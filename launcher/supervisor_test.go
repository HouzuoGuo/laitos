package launcher

import (
	"reflect"
	"strings"
	"testing"
)

func TestRemoveFromFlags(t *testing.T) {
	t.Run("remove bare flags", func(t *testing.T) {
		flags := []string{"front", "-aa=11", "middle", "-bb", "22", "tail"}
		if ret := RemoveFromFlags(func(s string) bool {
			return s == "front" || s == "middle" || s == "tail"
		}, flags); !reflect.DeepEqual(ret, []string{"-aa=11", "-bb", "22"}) {
			t.Fatal(ret)
		}
	})

	t.Run("remove -key=val flags", func(t *testing.T) {
		flags := []string{"-front=1", "aa", "-middle=2", "-bb", "22", "-tail=3"}
		if ret := RemoveFromFlags(func(s string) bool {
			return strings.HasPrefix(s, "-front") || strings.HasPrefix(s, "-middle") || strings.HasPrefix(s, "-tail")
		}, flags); !reflect.DeepEqual(ret, []string{"aa", "-bb", "22"}) {
			t.Fatal(ret)
		}
	})

	t.Run("remove -key val flags", func(t *testing.T) {
		flags := []string{"-front", "1", "aa", "-middle", "2", "-bb", "22", "-tail", "3"}
		if ret := RemoveFromFlags(func(s string) bool {
			return strings.HasPrefix(s, "-front") || strings.HasPrefix(s, "-middle") || strings.HasPrefix(s, "-tail")
		}, flags); !reflect.DeepEqual(ret, []string{"aa", "-bb", "22"}) {
			t.Fatal(ret)
		}
	})

	t.Run("remove empty val flags", func(t *testing.T) {
		flags := []string{"", "-front", "1", "aa", "", "-middle", "2", "-bb", "22", "-tail", "3", ""}
		if ret := RemoveFromFlags(func(s string) bool {
			return strings.HasPrefix(s, "-front") || strings.HasPrefix(s, "-middle") || strings.HasPrefix(s, "-tail")
		}, flags); !reflect.DeepEqual(ret, []string{"aa", "-bb", "22"}) {
			t.Fatal(ret)
		}
	})
}

func TestSupervisor_GetLaunchParameters(t *testing.T) {
	originalCLIFlags := []string{"-awslambda", "-awsinteg", "-debug=false", "-disableconflicts", "-tunesystem", "-swapoff", "-gomaxprocs", "16", "-config", "config.json", "-daemons", "httpd,maintenance,smtpd,telegram", "-profhttpport=15899"}
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
	if !reflect.DeepEqual(flags, []string{"-awslambda", "-awsinteg", "-debug=false", "-disableconflicts", "-tunesystem", "-swapoff", "-gomaxprocs", "16", "-config", "config.json", "-profhttpport=15899", "-supervisor=false", "-daemons", "httpd,maintenance,smtpd,telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, originalDaemonList) {
		t.Fatal(daemons)
	}

	// The second attempt should launch main program using reduced set of parameters, but same set of daemons.
	flags, daemons = sup.GetLaunchParameters(1)
	if !reflect.DeepEqual(flags, []string{"-awslambda", "-config", "config.json", "-supervisor=false", "-daemons", "httpd,maintenance,smtpd,telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, originalDaemonList) {
		t.Fatal(daemons)
	}

	// The third attempt should shed maintenance daemon
	flags, daemons = sup.GetLaunchParameters(2)
	if !reflect.DeepEqual(flags, []string{"-awslambda", "-config", "config.json", "-supervisor=false", "-daemons", "httpd,smtpd,telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, []string{"httpd", "smtpd", "telegram"}) {
		t.Fatal(daemons)
	}

	// The fourth attempt should shed SMTP daemon
	flags, daemons = sup.GetLaunchParameters(3)
	if !reflect.DeepEqual(flags, []string{"-awslambda", "-config", "config.json", "-supervisor=false", "-daemons", "httpd,telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, []string{"httpd", "telegram"}) {
		t.Fatal(daemons)
	}

	// The fifth attempt should shed HTTP daemon
	flags, daemons = sup.GetLaunchParameters(4)
	if !reflect.DeepEqual(flags, []string{"-awslambda", "-config", "config.json", "-supervisor=false", "-daemons", "telegram"}) {
		t.Fatal(flags)
	}
	if !reflect.DeepEqual(daemons, []string{"telegram"}) {
		t.Fatal(daemons)
	}

	// All further attempts should not shed any daemons, but only remove non-essential flags.
	for i := 5; i < 500; i++ {
		flags, daemons = sup.GetLaunchParameters(5)
		if !reflect.DeepEqual(flags, []string{"-awslambda", "-config", "config.json", "-supervisor=false", "-daemons", "httpd,maintenance,smtpd,telegram"}) {
			t.Fatal(flags)
		}
		if !reflect.DeepEqual(daemons, originalDaemonList) {
			t.Fatal(daemons)
		}
	}
}
