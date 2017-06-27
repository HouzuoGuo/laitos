package main

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/global"
	"io/ioutil"
	pseudoRand "math/rand"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

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
func StopConflictingDaemons() {
	if os.Getuid() != 0 {
		logger.Fatalf("StopConflictingDaemons", "", nil, "you must run laitos as root user if you wish to automatically disable conflicting daemons")
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
					if _, err := env.InvokeShell("/bin/sh", 5, cmd); err == nil {
						success = true
						// Continue to run subsequent commands to further disable the service
					}
				}
				// Do not overwhelm system with too many consecutive commands
				time.Sleep(1 * time.Second)
			}
			if success {
				logger.Printf("StopConflictingDaemons", name, nil, "the daemon has been successfully stopped and disabled")
			}
		}(name)
	}
	waitGroup.Wait()
}

// A daemon that starts and blocks.
type Daemon interface {
	StartAndBlock() error
}

// Start a daemon in a separate goroutine. If the daemon crashes, the goroutine logs an error message but does not crash the entire program.
func StartDaemon(counter *int32, waitGroup *sync.WaitGroup, name string, daemon Daemon) {
	atomic.AddInt32(counter, 1)
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		defer func() {
			if err := recover(); err != nil {
				logger.Warningf("main", name, errors.New(fmt.Sprint(err)), "daemon crashed!")
			}
		}()
		logger.Printf("main", name, nil, "going to start daemon")
		if err := daemon.StartAndBlock(); err != nil {
			logger.Warningf("main", name, err, "daemon failed")
			return
		}
	}()
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
	var configFile, frontend string
	var conflictFree, debug bool
	var gomaxprocs int
	flag.StringVar(&configFile, "config", "", "(Mandatory) path to configuration file in JSON syntax")
	flag.StringVar(&frontend, "frontend", "", "(Mandatory) comma-separated frontend services to start (dnsd, healthcheck, httpd, insecurehttpd, mailp, plaintext, smtpd, sockd, telegram)")
	flag.BoolVar(&conflictFree, "conflictfree", false, "(Optional) automatically stop and disable system daemons that may run into port conflict with laitos")
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

	// Deserialise JSON configuration file
	if configFile == "" {
		logger.Fatalf("main", "", nil, "please provide a configuration file (-config)")
		return
	}
	var config Config
	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Fatalf("main", "", err, "failed to read config file \"%s\"", configFile)
		return
	}
	if err := config.DeserialiseFromJSON(configBytes); err != nil {
		logger.Fatalf("main", "", err, "failed to deserialise config file \"%s\"", configFile)
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
	if conflictFree {
		StopConflictingDaemons()
	}

	// Finally laitos daemon start
	waitGroup := &sync.WaitGroup{}
	var numDaemons int32
	for _, frontendName := range frontends {
		// Daemons are started all at once, the order of startup does not matter.
		switch frontendName {
		case "dnsd":
			StartDaemon(&numDaemons, waitGroup, frontendName, config.GetDNSD())
		case "healthcheck":
			StartDaemon(&numDaemons, waitGroup, frontendName, config.GetHealthCheck())
		case "httpd":
			StartDaemon(&numDaemons, waitGroup, frontendName, config.GetHTTPD())
		case "insecurehttpd":
			StartDaemon(&numDaemons, waitGroup, frontendName, config.GetInsecureHTTPD())
		case "mailp":
			mailContent, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				logger.Fatalf("main", "", err, "failed to read mail from STDIN")
				return
			}
			if err := config.GetMailProcessor().Process(mailContent); err != nil {
				logger.Fatalf("main", "", err, "failed to process mail")
			}
		case "plaintext":
			StartDaemon(&numDaemons, waitGroup, frontendName, config.GetPlainTextDaemon())
		case "smtpd":
			StartDaemon(&numDaemons, waitGroup, frontendName, config.GetMailDaemon())
		case "sockd":
			StartDaemon(&numDaemons, waitGroup, frontendName, config.GetSockDaemon())
		case "telegram":
			StartDaemon(&numDaemons, waitGroup, frontendName, config.GetTelegramBot())
		default:
			logger.Fatalf("main", "", err, "unknown frontend name \"%s\"", frontendName)
		}
	}
	if numDaemons > 0 {
		logger.Printf("main", "", nil, "started %d daemons", numDaemons)
	}
	// Daemons are not really supposed to quit
	waitGroup.Wait()
}
