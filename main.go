package main

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"flag"
	"io/ioutil"
	"log"
	pseudoRand "math/rand"
	"os"
	"regexp"
	"sync"
	"syscall"
	"time"
)

// Re-seed global pseudo random generator using cryptographic random number generator.
func ReseedPseudoRand() {
	numAttempts := 1
	for ; ; numAttempts++ {
		seedBytes := make([]byte, 8)
		_, err := cryptoRand.Read(seedBytes)
		if err != nil {
			log.Panicf("ReseedPseudoRand: failed to read from crypto random generator - %v", err)
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
	log.Printf("ReseedPseudoRand: succeeded after %d attempts", numAttempts)
}

func main() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
			log.Fatalf("main: failed to lock memory - %v", err)
		}
		log.Print("Program has been locked into memory for safety reasons.")
	} else {
		log.Print("Program is not running as root (UID 0) hence memory is not locked, your private information will leak into swap.")
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
	flag.StringVar(&frontend, "frontend", "", "(Mandatory) comma-separated frontend services to start (httpd, mailp, telegram)")
	flag.Parse()

	if configFile == "" {
		log.Fatal("Please provide a configuration file (-config).")
	}
	frontendList := regexp.MustCompile(`\w+`)
	frontends := frontendList.FindAllString(frontend, -1)
	if frontends == nil || len(frontends) == 0 {
		log.Fatal("Please provide comma-separated list of frontend services to start (-frontend).")
	}

	// Deserialise configuration file
	var config Config
	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config file \"%s\" - %v", configFile, err)
	}
	if err := config.DeserialiseFromJSON(configBytes); err != nil {
		log.Fatalf("Failed to deserialise config file \"%s\" - %v", configFile, err)
	}

	// Start frontent daemons
	daemons := &sync.WaitGroup{}
	var hasDaemon bool
	for _, frontendName := range frontends {
		switch frontendName {
		case "httpd":
			hasDaemon = true
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				log.Print("main: going to start httpd")
				if err := config.GetHTTPD().StartAndBlock(); err != nil {
					log.Fatalf("main: failed to start http daemon - %v", err)
				}
			}()
		case "mailp":
			mailContent, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				log.Fatalf("main: failed to read mail content from stdin - %v", err)
			}
			if err := config.GetMailProcessor().Process(mailContent); err != nil {
				log.Fatalf("main: failed to process mail - %v", err)
			}
		case "telegram":
			hasDaemon = true
			daemons.Add(1)
			go func() {
				defer daemons.Done()
				log.Print("main: going to start telegram bot")
				if err := config.GetTelegramBot().StartAndBlock(); err != nil {
					log.Fatalf("main: failed to start telegram bot daemon - %v", err)
				}
			}()
		default:
			log.Fatalf("main: unknown frontend name - %s", frontendName)
		}
	}
	if hasDaemon {
		log.Printf("main: frontend daemons are started")
	}
	// Daemons are not really supposed to quit
	daemons.Wait()
}
