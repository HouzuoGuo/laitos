/*
websh is a comprehensive do-everything daemon that delivers many Internet features (not generic Internet Protocol) over
alternative communication infrastructure such as PSTN, GSM, and satellite.

You must exercise extreme caution when using this software program, inappropriate configuration will severely compromise
the security of the host computer. I am not responsible for any damage/potential damage caused to your computers.

Copyright (c) 2017, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/
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
	"syscall"
	"time"
)

// Re-seed global pseudo random generator using cryptographic random number generator.
func ReseedPseudoRand() {
	seedBytes := make([]byte, 8)
	_, err := cryptoRand.Read(seedBytes)
	if err != nil {
		log.Panicf("ReseedPseudoRand: failed to read from crypto random generator - %v", err)
	}
	seed, _ := binary.Varint(seedBytes)
	if seed == 0 {
		log.Panic("ReseedPseudoRand: binary conversion failed")
	}
	pseudoRand.Seed(seed)
	log.Print("ReseedPseudoRand: succeeded")
}

func main() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
			log.Panicf("main: failed to lock memory - %v", err)
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
	flag.StringVar(&frontend, "frontend", "", "(Mandatory) comma-separated frontend services to start (httpd, mailp)")
	flag.Parse()

	if configFile == "" {
		log.Panic("Please provide a configuration file (-config).")
	}
	frontendList := regexp.MustCompile(`\w+`)
	frontends := frontendList.FindAllString(frontend, -1)
	if frontends == nil || len(frontends) == 0 {
		log.Panic("Please provide comma-separated list of frontend services to start (-frontend).")
	}

	// Deserialise configuration file
	var config Config
	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Panicf("Failed to read config file \"%s\" - %v", configFile, err)
	}
	if err := config.DeserialiseFromJSON(configBytes); err != nil {
		log.Panicf("Failed to deserialise config file \"%s\" - %v", configFile, err)
	}

	for _, frontendName := range frontends {
		switch frontendName {
		case "httpd":
			go func() {
				if err := config.GetHTTPD().StartAndBlock(); err != nil {
					log.Panicf("main: failed to start http daemon - %v", err)
				}
			}()
		case "mailp":
			mailContent, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				log.Panicf("main: failed to read mail content from stdin - %v", err)
			}
			if err := config.GetMailProcessor().Process(mailContent); err != nil {
				log.Panicf("main: failed to process mail - %v", err)
			}
		}
	}
}
