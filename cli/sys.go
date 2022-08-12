package cli

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

/*
CopyNonEssentialUtilitiesInBackground immediately copies utility programs that are not essential but helpful to certain
toolbox features and daemons, and then continues in background at regular interval (1 hour).
*/
func CopyNonEssentialUtilitiesInBackground(logger lalog.Logger) {
	periodicCopy := &misc.Periodic{
		LogActorName: "copy-non-essential-utils",
		Interval:     1 * time.Hour,
		MaxInt:       1,
		Func: func(_ context.Context, _, _ int) error {
			platform.CopyNonEssentialUtilities(logger)
			logger.Info("CopyNonEssentialUtilitiesInBackground", "", nil, "successfully copied non-essential utility programs")
			return nil
		},
	}
	if err := periodicCopy.Start(context.Background()); err != nil {
		panic(err)
	}
}

// DisableConflicts prevents system daemons from conflicting with laitos, this is usually done by disabling them.
func DisableConflicts(logger lalog.Logger) {
	if !platform.HostIsWindows() && os.Getuid() != 0 {
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
			if platform.DisableStopDaemon(name) {
				logger.Info("DisableConflicts", name, nil, "the daemon has been successfully stopped and disabled")
			}
		}(name)
	}
	waitGroup.Wait()
	logger.Info("DisableConflicts", "systemd-resolved", nil, "%s", platform.DisableInterferingResolved())
}
