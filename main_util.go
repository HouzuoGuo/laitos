package main

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	pseudoRand "math/rand"
	_ "net/http/pprof"
	"os"
	"os/signal"
	runtimePprof "runtime/pprof"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/awsinteg"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

var (
	loggerSQSClientInitOnce = new(sync.Once)
)

// LogWarningCallbackQueueMessageBody contains details of a warning log entry, ready to be serialised into JSON for sending as an SQS message.
type LogWarningCallbackQueueMessageBody struct {
	UnixNanoSec   int64  `json:"unix_nano_sec"`
	UnixSec       int64  `json:"unix_sec"`
	ComponentName string `json:"component_name"`
	ComponentID   string `json:"component_id"`
	FunctionName  string `json:"function_name"`
	ActorName     string `json:"actor_name"`
	Error         error  `json:"error"`
	Message       string `json:"message"`
}

// GetJSON returns the message body serialised into JSON.
func (messageBody LogWarningCallbackQueueMessageBody) GetJSON() []byte {
	serialised, err := json.Marshal(messageBody)
	if err != nil {
		return []byte{}
	}
	return serialised
}

/*
InstallOptionalLoggerSQSCallback installs a global callback function for all laitos loggers to forward a copy of each warning
log entry to AWS SQS.
This behaviour is enabled optionally by specifying the queue URL in environment variable LAITOS_SEND_WARNING_LOG_TO_SQS_URL.
*/
func InstallOptionalLoggerSQSCallback() {
	sendWarningLogToSQSURL := os.Getenv("LAITOS_SEND_WARNING_LOG_TO_SQS_URL")
	if misc.EnableAWSIntegration && inet.IsAWS() && sendWarningLogToSQSURL != "" {
		logger.Info("InstallOptionalLoggerSQSCallback", "", nil, "installing callback for sending logger warning messages to SQS")
		loggerSQSClientInitOnce.Do(func() {
			sqsClient, err := awsinteg.NewSQSClient()
			if err != nil {
				lalog.DefaultLogger.Warning("InstallLoggerSQSCallback", "", err, "failed to initialise SQS client")
				return
			}
			// Give SQS a copy of each warning message
			lalog.GlobalLogWarningCallback = func(componentName, componentID, funcName, actorName string, err error, msg string) {
				// By contract, the function body must avoid generating a warning log message to avoid infinite recurison.
				sendTimeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				logMessageRecord := LogWarningCallbackQueueMessageBody{
					UnixNanoSec:   time.Now().UnixNano(),
					UnixSec:       time.Now().Unix(),
					ComponentName: componentName,
					ComponentID:   componentID,
					FunctionName:  funcName,
					ActorName:     actorName,
					Error:         err,
					Message:       msg,
				}
				_ = sqsClient.SendMessage(sendTimeoutCtx, sendWarningLogToSQSURL, string(logMessageRecord.GetJSON()))
			}
		})
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
			logger.Info("ReseedPseudoRandAndInBackground", "", nil, "successfully seeded RNG")
		}
	}()
}

/*
CopyNonEssentialUtilitiesInBackground immediately copies utility programs that are not essential but helpful to certain
toolbox features and daemons, and then continues in background at regular interval (1 hour).
*/
func CopyNonEssentialUtilitiesInBackground() {
	go func() {
		for {
			platform.CopyNonEssentialUtilities(logger)
			logger.Info("PrepareUtilitiesAndInBackground", "", nil, "successfully copied non-essential utility programs")
			time.Sleep(1 * time.Hour)
		}
	}()
}

// DisableConflicts prevents system daemons from conflicting with laitos, this is usually done by disabling them.
func DisableConflicts() {
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
		if err := fun(); err == nil {
			logger.Info("AutoRestart", logActorName, nil, "the function has returned successfully, no further restart is required.")
			return
		} else {
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
}
