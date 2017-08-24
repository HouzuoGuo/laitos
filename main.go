package main

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"io/ioutil"
	pseudoRand "math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const DaemonRestartIntervalSec = 10 // DaemonRestartIntervalSec is the interval to pause between daemon start attempts.

var logger = global.Logger{ComponentName: "laitos", ComponentID: strconv.Itoa(os.Getpid())}

// Re-seed global pseudo random generator using cryptographic random number generator.
func ReseedPseudoRand() {
	numAttempts := 1
	for ; ; numAttempts++ {
		seedBytes := make([]byte, 8)
		_, err := cryptoRand.Read(seedBytes)
		if err != nil {
			logger.Panicf("ReseedPseudoRand", "", err, "failed to read from random generator")
		}
		seed, _ := binary.Varint(seedBytes)
		if seed == 0 {
			// If random entropy decodes into an integer that overflows, simply retry.
			continue
		} else {
			pseudoRand.Seed(seed)
			break
		}
	}
	logger.Printf("ReseedPseudoRand", "", nil, "succeeded after %d attempt(s)", numAttempts)
}

// Stop and disable daemons that may run into port usage conflicts with laitos.
func DisableConflictingDaemons() {
	if os.Getuid() != 0 {
		logger.Fatalf("DisableConflictingDaemons", "", nil, "you must run laitos as root user if you wish to automatically disable conflicting daemons")
	}
	list := []string{"apache", "apache2", "bind", "bind9", "httpd", "lighttpd", "named", "named-chroot", "postfix", "sendmail"}
	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(len(list))
	for _, name := range list {
		go func(name string) {
			defer waitGroup.Done()
			var success bool
			// Disable+stop intensifies three times...
			for i := 0; i < 3; i++ {
				cmds := []string{
					// Some hosting providers still do not use systemd, an example is Amazon Elastic Beanstalk.
					fmt.Sprintf("/etc/init.d/%s stop", name),
					fmt.Sprintf("chkconfig %s off", name),
					fmt.Sprintf("chmod 0000 /etc/init.d/%s", name),

					fmt.Sprintf("systemctl stop %s", name),
					fmt.Sprintf("systemctl disable %s", name),
					fmt.Sprintf("systemctl mask %s", name),
				}
				for _, cmd := range cmds {
					if _, err := env.InvokeShell(5, "/bin/sh", cmd); err == nil {
						success = true
						// Continue to run subsequent commands to further disable the service
					}
				}
				// Do not overwhelm system with too many consecutive commands
				time.Sleep(1 * time.Second)
			}
			if success {
				logger.Printf("DisableConflictingDaemons", name, nil, "the daemon has been successfully stopped and disabled")
			}
		}(name)
	}
	waitGroup.Wait()
}

func main() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
			logger.Fatalf("main", "", err, "failed to lock memory")
			return
		}
		logger.Warningf("main", "", nil, "program has been locked into memory for safety reasons")
	} else {
		logger.Warningf("main", "", nil, "program is not running as root (UID 0) hence memory is not locked, your private information will leak into swap.")
	}

	// Process command line flags
	var frontend string
	var disableConflicts, tuneSystem, debug bool
	var gomaxprocs int
	flag.StringVar(&global.ConfigFilePath, "config", "", "(Mandatory) path to configuration file in JSON syntax")
	flag.StringVar(&frontend, "frontend", "", "(Mandatory) comma-separated frontend services to start (dnsd, httpd, insecurehttpd, mailp, maintenance, plaintext, smtpd, sockd, telegram)")
	flag.BoolVar(&disableConflicts, "disableconflicts", false, "(Optional) automatically stop and disable other daemon programs that may cause port usage conflicts")
	flag.BoolVar(&tuneSystem, "tunesystem", false, "(Optional) tune operating system parameters for optimal performance")
	flag.BoolVar(&debug, "debug", false, "(Optional) print goroutine stack traces upon receiving interrupt signal")
	flag.IntVar(&gomaxprocs, "gomaxprocs", 0, "(Optional) set gomaxprocs")
	flag.Parse()

	// Dump goroutine stacktraces upon receiving interrupt signal
	if debug {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			for range c {
				pprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
			}
		}()
	}

	if global.ConfigFilePath == "" {
		logger.Fatalf("main", "", nil, "please provide a configuration file (-config)")
		return
	}
	var err error
	global.ConfigFilePath, err = filepath.Abs(global.ConfigFilePath)
	if err != nil {
		logger.Fatalf("main", "", err, "failed to determine absolute path of config file \"%s\"", global.ConfigFilePath)
	}

	// Deserialise JSON configuration file
	var config Config
	configBytes, err := ioutil.ReadFile(global.ConfigFilePath)
	if err != nil {
		logger.Fatalf("main", "", err, "failed to read config file \"%s\"", global.ConfigFilePath)
		return
	}
	if err := config.DeserialiseFromJSON(configBytes); err != nil {
		logger.Fatalf("main", "", err, "failed to deserialise config file \"%s\"", global.ConfigFilePath)
		return
	}

	// Figure out what daemons are to be started
	frontendList := regexp.MustCompile(`\w+`)
	frontends := frontendList.FindAllString(frontend, -1)
	if frontends == nil || len(frontends) == 0 {
		logger.Fatalf("main", "", nil, "please provide comma-separated list of frontend services to start (-frontend).")
		return
	}

	// Re-seed pseudo random number generator once a while
	ReseedPseudoRand()
	go func() {
		time.Sleep(2 * time.Minute)
		ReseedPseudoRand()
	}()

	// Configure gomaxprocs to help laitos daemons
	if gomaxprocs > 0 {
		oldGomaxprocs := runtime.GOMAXPROCS(gomaxprocs)
		logger.Warningf("main", "", nil, "GOMAXPROCS has been changed from %d to %d", oldGomaxprocs, gomaxprocs)
	} else {
		logger.Warningf("main", "", nil, "GOMAXPROCS is unchanged at %d", runtime.GOMAXPROCS(0))
	}

	// Stop certain daemons to increase chance of successful launch of laitos daemons
	if disableConflicts {
		DisableConflictingDaemons()
	}

	// Tune operating system parameters for optimal performance and better resource utilisation
	if tuneSystem {
		logger.Warningf("main", "", nil, "System tuning result is: \n%s", feature.TuneLinux())
	}

	// Start each daemon
	for _, frontendName := range frontends {
		// Daemons are started asynchronously, the order of startup does not matter.
		switch frontendName {
		case "dnsd":
			go common.NewSupervisor(config.GetDNSD(), DaemonRestartIntervalSec, frontendName).Start()
		case "httpd":
			go common.NewSupervisor(config.GetHTTPD(), DaemonRestartIntervalSec, frontendName).Start()
		case "insecurehttpd":
			go common.NewSupervisor(config.GetInsecureHTTPD(), DaemonRestartIntervalSec, frontendName).Start()
		case "mailp":
			mailContent, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				logger.Fatalf("main", "", err, "failed to read mail from STDIN")
				return
			}
			if err := config.GetMailProcessor().Process(mailContent); err != nil {
				logger.Fatalf("main", "", err, "failed to process mail")
			}
			// Mail processor for standard input is not a daemon
			return
		case "maintenance":
			go common.NewSupervisor(config.GetMaintenance(), DaemonRestartIntervalSec, frontendName).Start()
		case "plaintext":
			go common.NewSupervisor(config.GetPlainTextDaemon(), DaemonRestartIntervalSec, frontendName).Start()
		case "smtpd":
			go common.NewSupervisor(config.GetMailDaemon(), DaemonRestartIntervalSec, frontendName).Start()
		case "sockd":
			go common.NewSupervisor(config.GetSockDaemon(), DaemonRestartIntervalSec, frontendName).Start()
		case "telegram":
			go common.NewSupervisor(config.GetTelegramBot(), DaemonRestartIntervalSec, frontendName).Start()
		default:
			logger.Fatalf("main", "", err, "unknown frontend name \"%s\"", frontendName)
		}
	}
	// Daemons are not really supposed to quit.
	// In rare circumstance, if all daemons fail to start without panicking, laitos will just hang right here.
	for {
		time.Sleep(time.Hour)
		logger.Printf("main", "", nil, "laitos has been up for %s", time.Now().Sub(global.StartupTime))
	}
}
