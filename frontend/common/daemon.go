package common

import (
	"errors"
	"github.com/HouzuoGuo/laitos/global"
	"time"
)

var ErrDaemonCrash = errors.New("Daemon crashed") // ErrDaemonCrash is returned by startAndRecover when daemon crashes.

// Daemon is capable of starting - a blocking action, and stopping.
type Daemon interface {
	StartAndBlock() error
	Stop()
}

// Supervisor supervises a daemon by starting the daemon, and restarts the daemon should it crash.
type Supervisor struct {
	RestartIntervalSec int // RestartIntervalSec is the delay between recovering daemon's panic and restarting the daemon.

	daemon Daemon        // daemon is the one being supervised.
	logger global.Logger // logger logs panics made by daemon.
}

// NewSupervisor constructs a daemon's supervisor.
func NewSupervisor(daemon Daemon, restartIntervalSec int, componentName string) *Supervisor {
	return &Supervisor{
		RestartIntervalSec: restartIntervalSec,
		daemon:             daemon,
		logger:             global.Logger{ComponentName: componentName},
	}
}

/*
startAndRecover starts the daemon. Should the start function panic, the panic is recovered and logged, the function
returns ErrDaemonCrash. Otherwise, it returns daemon's startup error if there is any.
*/
func (super *Supervisor) startAndRecover() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrDaemonCrash
			super.logger.Warningf("startAndRecover", "supervisor", nil, "daemon crashed - %v", r)
		}
	}()
	err = super.daemon.StartAndBlock()
	return
}

// stopAndRecover stops a daemon. Should the stop function panic, the panic is recovered and logged.
func (super *Supervisor) stopAndRecover() {
	defer func() {
		if r := recover(); r != nil {
			super.logger.Warningf("stopAndRecover", "supervisor", nil, "failed to stop daemon - %v", r)
		}
	}()
	super.daemon.Stop()
}

/*
Supervise starts a daemon and restarts in a second it if it crashes. If daemon's startup routine fails but does not
result in a panic, the failure is returned.
*/
func (super *Supervisor) Start() error {
	for {
		super.logger.Warningf("Start", "supervisor", nil, "attempting to start daemon")
		err := super.startAndRecover()
		if err != nil && err != ErrDaemonCrash {
			// A daemon startup error that is not a crash
			return err
		}
		if err == nil {
			super.logger.Warningf("Start", "supervisor", nil, "daemon quit without an error, restarting in %s seconds", super.RestartIntervalSec)
		} else {
			super.logger.Warningf("Start", "supervisor", nil, "restarting panicked daemon in %d seconds", super.RestartIntervalSec)
		}
		super.stopAndRecover()
		time.Sleep(time.Duration(super.RestartIntervalSec) * time.Second)
		// Stop it again, just in case.
		super.stopAndRecover()
	}
}
