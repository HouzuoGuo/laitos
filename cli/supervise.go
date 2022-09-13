package cli

import (
	"bufio"
	"context"
	cryptoRand "crypto/rand"
	"encoding/binary"
	pseudoRand "math/rand"
	"os"
	"os/signal"
	"path/filepath"
	runtimePprof "runtime/pprof"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/launcher/passwdserver"
	"github.com/HouzuoGuo/laitos/misc"
)

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
			logger.Warning(logActorName, nil, "emergency lock-down has been activated, no further restart is performed.")
			return
		}
		err := fun()
		if err == nil {
			logger.Info(logActorName, nil, "the function has returned successfully, no further restart is required.")
			return
		}
		if delaySec == 0 {
			logger.Warning(logActorName, err, "restarting immediately")
		} else {
			logger.Warning(logActorName, err, "restarting in %d seconds", delaySec)
		}
		time.Sleep(time.Duration(delaySec) * time.Second)
		if delaySec < 60 {
			delaySec += 10
		}
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

// ReseedPseudoRandAndInBackground seeds the default PRNG using a
// cryptographically-secure RNG, and then spawns a background goroutine to
// continuously reseeds the default PRNG at regular interval.
// This function helps securing several laitos program components that depend on
// the default PRNG, therefore, it should be invoked at or near the start of the
// main function.
func ReseedPseudoRandAndInBackground(logger lalog.Logger) {
	// Avoid using misc.Periodic, for it uses the PRNG internally.
	reseedFun := func() {
		seedBytes := make([]byte, 8)
		_, err := cryptoRand.Read(seedBytes)
		if err != nil {
			logger.Abort("", err, "failed to read from random generator")
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
			time.Sleep(5 * time.Minute)
			reseedFun()
			logger.Info("", nil, "successfully re-seeded PRNG")
		}
	}()
}

// GetConfig returns the laitos program configuration content (JSON) by
// retrieving it from program environment, or a text file.
// If the text file is encrypted, the function will retrieve its encryption
// password from STDIN, password unlocking server, or a web server for password
// input, and then return the text file decrypted.
func GetConfig(logger lalog.Logger, pwdServer bool, pwdServerPort int, pwdServerURL string, passwordUnlockServers string) []byte {
	configBytes := []byte(strings.TrimSpace(os.Getenv("LAITOS_CONFIG")))
	if len(configBytes) == 0 {
		// Proceed to read the config file
		if misc.ConfigFilePath == "" {
			logger.Abort(nil, nil, "please provide a configuration file (-config)")
			return nil
		}
		var err error
		misc.ConfigFilePath, err = filepath.Abs(misc.ConfigFilePath)
		if err != nil {
			logger.Abort(nil, err, "failed to determine absolute path of config file \"%s\"", misc.ConfigFilePath)
			return nil
		}
		// If config file is encrypted, read its password from standard input.
		var isEncrypted bool
		configBytes, isEncrypted, err = misc.IsEncrypted(misc.ConfigFilePath)
		if err != nil {
			logger.Abort(nil, err, "failed to read configuration file \"%s\"", misc.ConfigFilePath)
			return nil
		}
		if isEncrypted {
			logger.Info(nil, nil, "the configuration file is encrypted, please pipe or type decryption password followed by Enter (new-line).")
			// There are multiple ways to collect the decryption password
			passwdRPCContext, passwdRPCCancel := context.WithCancel(context.Background())
			passwordCollectionServer := passwdserver.WebServer{
				Port: pwdServerPort,
				URL:  pwdServerURL,
			}
			if password := strings.TrimSpace(os.Getenv(misc.EnvironmentDecryptionPassword)); password != "" {
				logger.Info(nil, nil, "got decryption password of %d characters from environment variable %s", len(password), misc.EnvironmentDecryptionPassword)
				misc.ProgramDataDecryptionPasswordInput <- password
			} else {
				go func() {
					// Collect program data decryption password from STDIN, there is not an explicit cancellation for the buffered read.
					stdinReader := bufio.NewReader(os.Stdin)
					pwdFromStdin, err := stdinReader.ReadString('\n')
					if err == nil {
						logger.Info(nil, nil, "got decryption password from stdin")
						misc.ProgramDataDecryptionPasswordInput <- strings.TrimSpace(pwdFromStdin)
					} else {
						logger.Warning(nil, err, "failed to read decryption password from STDIN")
					}
				}()
				go func() {
					// Collect program data decryption password from gRPC servers dedicated to this purpose
					if passwordUnlockServers != "" {
						serverAddrs := strings.Split(passwordUnlockServers, ",")
						if password := GetUnlockingPasswordWithRetry(passwdRPCContext, true, logger, serverAddrs...); password != "" {
							misc.ProgramDataDecryptionPasswordInput <- password
						}
					}
				}()
				if pwdServer {
					// The web server launched here is distinct from the regular HTTP daemon. The sole purpose of the web server
					// is to present a web page to visitor for them to enter decryption password for program config and data files.
					// On Amazon ElasitcBeanstalk, application update cannot reliably kill the old program prior to launching
					// the new version, which means the web server often runs into port conflicts when its updated version starts
					// up. AutoRestart function helps to restart the server in such case.
					go AutoRestart(logger, "pwdserver", passwordCollectionServer.Start)
				}
				// The AWS lambda handler is also able to retrieve the password from API gateway stage configuration and place it into the channel.
			}
			plainTextPassword := <-misc.ProgramDataDecryptionPasswordInput
			misc.ProgramDataDecryptionPassword = plainTextPassword
			// Explicitly stop background routines that may be still trying to obtain a decryption password
			passwdRPCCancel()
			passwordCollectionServer.Shutdown()
			if configBytes, err = misc.Decrypt(misc.ConfigFilePath, misc.ProgramDataDecryptionPassword); err != nil {
				logger.Abort(nil, err, "failed to decrypt config file")
				return nil
			}
		}
	} else {
		logger.Info(nil, nil, "reading %d bytes of JSON configuration from environment variable LAITOS_CONFIG", len(configBytes))
	}
	return configBytes
}
