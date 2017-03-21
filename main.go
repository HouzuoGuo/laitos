package main

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"flag"
	"github.com/HouzuoGuo/laitos/lalog"
	"io/ioutil"
	pseudoRand "math/rand"
	"os"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var logger = lalog.Logger{ComponentName: "laitos", ComponentID: strconv.Itoa(os.Getpid())}

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

func main() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
			logger.Fatalf("main", "", err, "failed to lock memory")
			return
		}
		logger.Printf("main", "", nil, "program has been locked into memory for safety reasons")
	} else {
		logger.Printf("main", "", nil, "program is not running as root (UID 0) hence memory is not locked, your private information will leak into swap.")
	}

	// Re-seed pseudo random number generator once a while
	ReseedPseudoRand()
	go func() {
		ReseedPseudoRand()
		time.Sleep(2 * time.Minute)
	}()

	// Process command line flags
	var configFile, frontend string
	flag.StringVar(&configFile, "config", "", "(Mandatory) path to configuration file in JSON syntax")
	flag.StringVar(&frontend, "frontend", "", "(Mandatory) comma-separated frontend services to start (dnsd, healthcheck, httpd, httpd80, mailp, smtpd, sockd, telegram)")
	flag.Parse()

	if configFile == "" {
		logger.Fatalf("main", "", nil, "please provide a configuration file (-config)")
		return
	}
	frontendList := regexp.MustCompile(`\w+`)
	frontends := frontendList.FindAllString(frontend, -1)
	if frontends == nil || len(frontends) == 0 {
		logger.Fatalf("main", "", nil, "please provide comma-separated list of frontend services to start (-frontend).")
		return
	}

	// Deserialise configuration file
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

	// Start frontent daemons
	daemons := &sync.WaitGroup{}
	var numDaemons int
	for _, frontendName := range frontends {
		switch frontendName {
		case "dnsd":
			numDaemons++
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				logger.Printf("main", "", nil, "going to start dns daemon")
				if err := config.GetDNSD().StartAndBlock(); err != nil {
					logger.Fatalf("main", "", err, "failed to start dns daemon")
					return
				}
			}()
		case "healthcheck":
			numDaemons++
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				logger.Printf("main", "", nil, "going to start health check")
				if err := config.GetHealthCheck().StartAndBlock(); err != nil {
					logger.Fatalf("main", "", err, "failed to start health check")
					return
				}
			}()
		case "httpd":
			numDaemons++
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				logger.Printf("main", "", nil, "going to start http daemon")
				if err := config.GetHTTPD().StartAndBlock(); err != nil {
					logger.Fatalf("main", "", err, "failed to start http daemon")
					return
				}
			}()
		case "httpd80":
			numDaemons++
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				logger.Printf("main", "", nil, "going to start http80 daemon")
				if err := config.GetHTTPD80().StartAndBlock(); err != nil {
					logger.Fatalf("main", "", err, "failed to start http80 daemon")
					return
				}
			}()
		case "mailp":
			mailContent, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				logger.Fatalf("main", "", err, "failed to read mail from STDIN")
				return
			}
			if err := config.GetMailProcessor().Process(mailContent); err != nil {
				logger.Fatalf("main", "", err, "failed to process mail")
			}
		case "smtpd":
			numDaemons++
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				logger.Printf("main", "", nil, "going to start smtp daemon")
				if err := config.GetMailDaemon().StartAndBlock(); err != nil {
					logger.Fatalf("main", "", err, "failed to start smtp daemon")
					return
				}
			}()
		case "sockd":
			numDaemons++
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				logger.Printf("main", "", nil, "going to start sock daemon")
				if err := config.GetSockDaemon().StartAndBlock(); err != nil {
					logger.Fatalf("main", "", err, "failed to start sock daemon")
					return
				}
			}()
		case "telegram":
			numDaemons++
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				logger.Printf("main", "", nil, "going to start telegram bot")
				if err := config.GetTelegramBot().StartAndBlock(); err != nil {
					logger.Fatalf("main", "", err, "failed to start telegram bot")
					return
				}
			}()
		default:
			logger.Fatalf("main", "", err, "unknown frontend name \"%s\"", frontendName)
		}
	}
	if numDaemons > 0 {
		logger.Printf("main", "", nil, "started %d daemons", numDaemons)
	}
	// Daemons are not really supposed to quit
	daemons.Wait()
}
