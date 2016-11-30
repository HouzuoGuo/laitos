/*
A simple do-everything daemon, primary for offering control of your computer via telephone and SMS.

The program can run in two modes:
- HTTPS daemon mode, secured by endpoint port number + endpoint name + PIN.
- Mail processing mode (~/.forward), secured by your username + PIN.

To call the service from command line client, run:
curl -v 'https://localhost:12321/my_secret_endpoint_name' --data-ascii 'Body=MYSECRETecho hello world'

Please note: exercise extreme caution when using this software program, inappropriate configuration will make your computer easily compromised! If you choose to use this program, I will not be responsible for any damage/potential damage caused to your computers.

Copyright (c) 2016, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"syscall"
	"time"
)

func main() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to lock memory - %v", err)
			os.Exit(111)
		}
		log.Print("Program is now locked into memory for safety reasons")
	}
	// Random delay in command processor uses this random number generator
	rand.Seed(time.Now().UnixNano())

	var configFilePath string
	var mailMode bool
	flag.StringVar(&configFilePath, "configfilepath", "", "Path to the configuration file")
	flag.BoolVar(&mailMode, "mailmode", false, "True if the program is processing an incoming mail, false if the program is running as a daemon")
	flag.Parse()

	// Load configuration file from CLI parameter
	if configFilePath == "" {
		flag.PrintDefaults()
		log.Panic("Please provide path to configuration file")
	}
	configContent, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Panic("Failed to read config file")
	}
	var conf Config
	if err = json.Unmarshal(configContent, &conf); err != nil {
		log.Panic("Failed to unmarshal config JSON")
	}

	if mailMode {
		// Process incoming mail from stdin
		mailProc := conf.ToMailProcessor()
		if err := mailProc.CheckConfig(); err != nil {
			log.Panic(err)
		}
		mailContent, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Panic("Failed to read mail content from STDIN")
		}
		mailProc.RunCommandFromMail(string(mailContent))
	} else {
		// Run web server and block until exit
		webServer := conf.ToWebServer()
		if err := webServer.CheckConfig(); err != nil {
			log.Panic(err)
		}
		if webServer.Command.Mailer.IsEnabled() {
			log.Printf("Will send mail notifications to %v", webServer.Command.Mailer.Recipients)
		} else {
			log.Print("Will not send mail notifications")
		}
		webServer.Run() // blocks
	}

}
