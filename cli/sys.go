package cli

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

const (
	// AppEngineDataDir is the relative path to a data directory that contains config files and data files required for launching laitos program
	// on GCP app engine.
	AppEngineDataDir = "./gcp_appengine_data"
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

// GAEDaemonList changes the PWD to App Engine's data directory and then returns
// the new comma-separated daemon list from the daemonList file.
// If laitos is not running on App Engine then the function does nothing and
// returns an empty string.
func GAEDaemonList(logger lalog.Logger) string {
	if os.Getenv("GAE_ENV") == "standard" {
		misc.EnablePrometheusIntegration = true
		// Change working directory to the data directory (if not done yet).
		// All program config files and data files are expected to reside in the data directory.
		cwd, err := os.Getwd()
		if err != nil {
			logger.Abort("main", "", err, "failed to determine current working directory")
		}
		if path.Base(cwd) != path.Base(AppEngineDataDir) {
			if err := os.Chdir(AppEngineDataDir); err != nil {
				logger.Abort("main", "", err, "failed to change directory to %s", AppEngineDataDir)
				return ""
			}
		}
		// Read the value of CLI parameter "-daemons" from a text file
		daemonListContent, err := ioutil.ReadFile("daemonList")
		if err != nil {
			logger.Abort("main", "", err, "failed to read daemonList")
			return ""
		}
		// Find program configuration data (encrypted or otherwise) in "config.json"
		misc.ConfigFilePath = "config.json"
		return string(daemonListContent)
	}
	return ""
}

// StartProfilingServer starts an HTTP server on localhost to serve program profiling data
func StartProfilingServer(logger lalog.Logger, pprofHTTPPort int) {
	if pprofHTTPPort > 0 {
		go func() {
			// Expose the entire selection of profiling profiles identical to the ones installed by pprof standard library package
			pprofMux := http.NewServeMux()
			pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
			pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
			logger.Info("main", "pprof", nil, "serving program profiling data over HTTP server on port %d", pprofHTTPPort)
			if err := http.ListenAndServe(net.JoinHostPort("localhost", strconv.Itoa(pprofHTTPPort)), pprofMux); err != nil {
				// This server is not expected to shutdown
				logger.Warning("main", "pprof", err, "failed to start HTTP server for program profiling data")

			}
		}()
	}
}
