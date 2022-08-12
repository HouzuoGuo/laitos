package cli

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	pseudoRand "math/rand"
	"os"
	"os/signal"
	runtimePprof "runtime/pprof"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

/*
AutoRestartFunc runs the input function and restarts it when it returns an error, subjected to increasing delay of up to 60 seconds
between each restart.
If the input function crashes in a panic, there won't be an auto-restart.
The function returns to the caller only after the input function returns nil.
*/
func AutoRestart(logger lalog.Logger, logActorName string, fun func() error) {
	delaySec := 0
	for {
		if misc.EmergencyLockDown {
			logger.Warning("AutoRestart", logActorName, nil, "emergency lock-down has been activated, no further restart is performed.")
			return
		}
		err := fun()
		if err == nil {
			logger.Info("AutoRestart", logActorName, nil, "the function has returned successfully, no further restart is required.")
			return
		}
		if delaySec == 0 {
			logger.Warning("AutoRestart", logActorName, err, "restarting immediately")
		} else {
			logger.Warning("AutoRestart", logActorName, err, "restarting in %d seconds", delaySec)
		}
		time.Sleep(time.Duration(delaySec) * time.Second)
		if delaySec < 60 {
			delaySec += 10
		}
	}
}

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

// ReseedPseudoRandAndInBackground seeds the default PRNG using a
// cryptographically-secure RNG, and then spawns a background goroutine to
// continuously reseeds the default PRNG at regular interval.
// This function helps securing several laitos program components that depend on
// the default PRNG, therefore, it should be invoked at or near the start of the
// main function.
func ReseedPseudoRandAndInBackground(logger lalog.Logger) {
	// Avoid using misc.Periodic, for it uses the PRNG internally.
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
			time.Sleep(5 * time.Minute)
			reseedFun()
			logger.Info("ReseedPseudoRandAndInBackground", "", nil, "successfully re-seeded PRNG")
		}
	}()
}
