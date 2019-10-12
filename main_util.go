package main

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	pseudoRand "math/rand"
	_ "net/http/pprof"
	"os"
	"os/signal"
	runtimePprof "runtime/pprof"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
)

// DumpGoroutinesOnInterrupt installs an interrupt signal handler that dumps all goroutine traces to standard error.
func DumpGoroutinesOnInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			_ = runtimePprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
		}
	}()
}

/*
ReseedPseudoRandAndContinue immediately re-seeds PRNG using cryptographic RNG, and then continues in background at
regular interval (3 minutes). This helps some laitos daemons that use the common PRNG instance for their operations.
*/
func ReseedPseudoRandAndInBackground() {
	reseedFun := func() {
		seedBytes := make([]byte, 8)
		_, err := cryptoRand.Read(seedBytes)
		if err != nil {
			logger.Abort("ReseedPseudoRandAndInBackground", "", err, "failed to read from random generator")
		}
		seed, _ := binary.Varint(seedBytes)
		if seed <= 0 {
			// If the random entropy fails to decode into an integer, seed PRNG with the system time.
			pseudoRand.Seed(time.Now().UnixNano())
		} else {
			pseudoRand.Seed(seed)
		}
	}
	reseedFun()
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			reseedFun()
			logger.Info("ReseedPseudoRandAndInBackground", "", nil, "has reseeded just now")
		}
	}()
}

/*
PrepareUtilitiesAndInBackground immediately copies utility programs that are not essential but helpful to certain
toolbox features and daemons, and then continues in background at regular interval (1 hour).

*/
func PrepareUtilitiesAndInBackground() {
	misc.PrepareUtilities(logger)
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			misc.PrepareUtilities(logger)
			logger.Info("PrepareUtilitiesAndInBackground", "", nil, "has run PrepareUtilities just now")
		}
	}()
}

// DisableConflicts prevents system daemons from conflicting with laitos, this is usually done by disabling them.
func DisableConflicts() {
	if !misc.HostIsWindows() && os.Getuid() != 0 {
		// Sorry, I do not know how to detect administrator privilege on Windows.
		logger.Abort("DisableConflicts", "", nil, "you must run laitos as root user if you wish to automatically disable system conflicts")
	}
	// All of these names are Linux services
	// Do not stop nginx for Linux, because Amazon ElasticBeanstalk uses it to receive and proxy web traffic.
	list := []string{"apache", "apache2", "bind", "bind9", "exim4", "httpd", "lighttpd", "named", "named-chroot", "postfix", "sendmail"}
	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(len(list))
	for _, name := range list {
		go func(name string) {
			defer waitGroup.Done()
			if misc.DisableStopDaemon(name) {
				logger.Info("DisableConflicts", name, nil, "the daemon has been successfully stopped and disabled")
			}
		}(name)
	}
	waitGroup.Wait()

	if relevant, out := misc.DisableInterferingResolved(); relevant {
		logger.Info("DisableConflicts", "systemd-resolved", nil, "attempted to disable resolved: %s", out)
	} else {
		logger.Info("DisableConflicts", "systemd-resolved", nil, "will not touch name resolution settings as resolved is not active")
	}
}
