package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/hzgl"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/launcher"
	"github.com/HouzuoGuo/laitos/launcher/passwdserver"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

const (
	ProfilerHTTPPort = 19151 // ProfilerHTTPPort is to be listened by net/http/pprof HTTP server when benchmark is turned on
)

var logger = lalog.Logger{ComponentName: "main", ComponentID: []lalog.LoggerIDField{{Key: "PID", Value: os.Getpid()}}}

/*
DecryptFile is a distinct routine of laitos main program, it reads password from standard input and uses it to decrypt the
input file in-place.
*/
func DecryptFile(filePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter password to decrypt file (no echo):")
	platform.SetTermEcho(false)
	password, _, err := reader.ReadLine()
	platform.SetTermEcho(true)
	if err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to read password")
		return
	}
	content, err := misc.Decrypt(filePath, []byte(password))
	if err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to decrypt file")
		return
	}
	if err := ioutil.WriteFile(filePath, content, 0600); err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to decrypt file")
		return
	}
	lalog.DefaultLogger.Info("DecryptFile", "main", nil, "successfully decrypte the file")
}

/*
EncryptFile is a distinct routine of laitos main program, it reads password from standard input and uses it to encrypt
the input file in-place.
*/
func EncryptFile(filePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter a password to encrypt the archive (no echo):")
	platform.SetTermEcho(false)
	password, _, err := reader.ReadLine()
	platform.SetTermEcho(true)
	if err != nil {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "failed to read password")
		return
	}
	password = []byte(strings.TrimSpace(string(password)))
	if err := misc.Encrypt(filePath, []byte(password)); err != nil {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "failed to encrypt file")
		return
	}
}

/*
StartPasswordWebServer is a distinct routine of laitos main program, it starts a simple web server to accept a password
input in order to decrypt laitos program data and launch the daemons.
*/
func StartPasswordWebServer(port int, url string) {
	ws := passwdserver.WebServer{
		Port: port,
		URL:  url,
	}
	/*
		On Amazon ElasitcBeanstalk, application update cannot reliably kill the old program prior to launching the new
		version, which means the web server often runs into port conflicts. Therefore, keep trying for 3 minutes.
	*/
	for i := 0; i < 30; i++ {
		if err := ws.Start(); err == nil {
			// Upon success, wait almost indefinitely (~5 years) because this is the main routine of this CLI action.
			time.Sleep(5 * 365 * 24 * time.Hour)
		} else {
			// Retry upon failure
			logger.Info("StartPasswordWebServer", "main", err, "failed to start the web server (attempt %d)", i)
			time.Sleep(6 * time.Second)
		}
	}
	logger.Abort("StartPasswordWebServer", "main", nil, "failed to start the web server after many attempts")
}

// terminatedDaemonToError wraps the daemon name and daemon's return value into an error.
func terminatedDaemonToError(daemonName string, daemonReturnValue error) error {
	return fmt.Errorf("Daemon \"%s\" has quit with error \"%+v\"", daemonName, daemonReturnValue)
}

/*
main runs one of several distinct routines as dictated by input command line flags:

- Utilities for maintaining encrypted program data archive (-datautil=extract|archive).

- Data unlocker (password input server) that accepts a password input to launch laitos daemons in supervisor mode by decrypting its
  program data (-pwdserver & -pwdserverport= & -pwdserverurl=).

- Supervisor runs laitos daemons in a seperate process and re-launches them in case of crash. Supervisor is turned on by
  default (-supervisor=true).

- Whether launched by supervisor or launched independently, start daemons (-config= & -daemons=).

- Benchmark routine that runs after daemons have been launched.
*/
func main() {
	hzgl.HZGL()
	// Process command line flags
	var daemonList string
	var disableConflicts, debug, benchmark bool
	var gomaxprocs int
	flag.StringVar(&misc.ConfigFilePath, launcher.ConfigFlagName, "", "(Mandatory) path to configuration file in JSON syntax")
	flag.StringVar(&daemonList, launcher.DaemonsFlagName, "", "(Mandatory) comma-separated daemons to start (autounlock, dnsd, httpd, insecurehttpd, maintenance, plainsocket, serialport, simpleipsvcd, smtpd, snmpd, sockd, telegram)")
	flag.BoolVar(&disableConflicts, "disableconflicts", false, "(Optional) automatically stop and disable other daemon programs that may cause port usage conflicts")
	flag.BoolVar(&debug, "debug", false, "(Optional) print goroutine stack traces upon receiving interrupt signal")
	flag.BoolVar(&benchmark, "benchmark", false, fmt.Sprintf("(Optional) continuously run benchmark routines on active daemons while exposing net/http/pprof on port %d", ProfilerHTTPPort))
	flag.IntVar(&gomaxprocs, "gomaxprocs", 0, "(Optional) set gomaxprocs")
	// Data unlocker (password input server) flags
	var pwdServer bool
	var pwdServerPort int
	var pwdServerURL string
	flag.BoolVar(&pwdServer, passwdserver.CLIFlag, false, "(Optional) launch web server to accept password for decrypting encrypted program data")
	flag.IntVar(&pwdServerPort, passwdserver.CLIFlag+"port", 80, "(Optional) port number of the password web server")
	flag.StringVar(&pwdServerURL, passwdserver.CLIFlag+"url", "", "(Optional) password input URL")
	// Data encryption utility flags
	var dataUtil, dataUtilFile string
	flag.StringVar(&dataUtil, "datautil", "", "(Optional) program data encryption utility: encrypt|decrypt")
	flag.StringVar(&dataUtilFile, "datautilfile", "", "(Optional) program data encryption utility: encrypt/decrypt file location")
	// Internal supervisor flag
	var isSupervisor = true
	flag.BoolVar(&isSupervisor, launcher.SupervisorFlagName, true, "(Internal use only) enter supervisor mode")

	flag.Parse()

	// Common diagnosis and security practices
	platform.LockMemory()
	ReseedPseudoRandAndInBackground()
	if debug {
		DumpGoroutinesOnInterrupt()
	}

	// ========================================================================
	// Utility routines - maintain encrypted laitos program data, no need to run any daemon.
	// ========================================================================
	if dataUtil != "" {
		if dataUtilFile == "" {
			logger.Abort("main", "", nil, "please provide data utility target file in parameter \"-datautilfile\"")
			return
		}
		switch dataUtil {
		case "encrypt":
			EncryptFile(dataUtilFile)
		case "decrypt":
			DecryptFile(dataUtilFile)
		default:
			logger.Abort("main", "", nil, "please provide mode of operation (encrypt|decrypt) for parameter \"-datautil\"")
		}
		return
	}

	// ========================================================================
	// Password input web server - start the web server to accept password input for decrypting program data.
	// ========================================================================
	if pwdServer {
		StartPasswordWebServer(pwdServerPort, pwdServerURL)
		return
	}
	/*
		If the password web server succeeded in decrypting program data, it will launch laitos daemons under supervisor;
		if the server is not relevant/involved in user's deployment, the user may simply ignore its program flags and
		launch laitos daemons right away.
		Be ware that supervisor is always turned on by default.
		Here come the preparation for both supervisor and daemons:
	*/
	// Read configuration JSON file
	if misc.ConfigFilePath == "" {
		logger.Abort("main", "", nil, "please provide a configuration file (-config)")
		return
	}
	var err error
	misc.ConfigFilePath, err = filepath.Abs(misc.ConfigFilePath)
	if err != nil {
		logger.Abort("main", "", err, "failed to determine absolute path of config file \"%s\"", misc.ConfigFilePath)
		return
	}
	// If config file is encrypted, read its password from standard input.
	configBytes, isEncrypted, err := misc.IsEncrypted(misc.ConfigFilePath)
	if err != nil {
		logger.Abort("main", "", err, "failed to read configuration file \"%s\"", misc.ConfigFilePath)
		return
	}
	if isEncrypted {
		logger.Info("main", "", nil, "the configuration file is encrypted, please pipe or type decryption password followed by Enter (new-line).")
		pwdReader := bufio.NewReader(os.Stdin)
		pwd, err := pwdReader.ReadString('\n')
		misc.UniversalDecryptionKey = []byte(strings.TrimSpace(pwd))
		if err != nil {
			logger.Abort("main", "", err, "failed to read password from stdin")
			return
		}
		if configBytes, err = misc.Decrypt(misc.ConfigFilePath, misc.UniversalDecryptionKey); err != nil {
			logger.Abort("main", "", err, "failed to decrypt config file")
			return
		}
	}

	var config launcher.Config
	/*
		Certain features (such as browser-in-browser and line oriented browser) rely on utilities in order to
		initialise, therefore prepare the non-essential utilities (which will prepare phantomJS among others) before
		deserialising and initialising configuration.
	*/
	PrepareUtilitiesAndInBackground()
	if err := config.DeserialiseFromJSON(configBytes); err != nil {
		logger.Abort("main", "", err, "failed to deserialise/initialise config file \"%s\"", misc.ConfigFilePath)
		return
	}
	// Figure out what daemons are to be started
	daemonNames := regexp.MustCompile(`\w+`).FindAllString(daemonList, -1)
	if len(daemonNames) == 0 {
		logger.Abort("main", "", nil, "please provide comma-separated list of daemon services to start (-daemons).")
		return
	}
	// Make sure all daemon names are valid
	for _, daemonName := range daemonNames {
		var found bool
		for _, goodName := range launcher.AllDaemons {
			if daemonName == goodName {
				found = true
			}
		}
		if !found {
			logger.Abort("main", "", err, "unknown daemon name \"%s\"", daemonName)
		}
	}

	// ========================================================================
	// Supervisor routine - launch an independent laitos process to run daemons.
	// The command line flag is turned on by default so that laitos daemons are
	// always protected by the supervisor by default. There is no good reason
	// for a user to turn it off manually.
	// ========================================================================
	if isSupervisor {
		supervisor := &launcher.Supervisor{
			CLIFlags:               os.Args[1:],
			NotificationRecipients: config.SupervisorNotificationRecipients,
			MailClient:             config.MailClient,
			DaemonNames:            daemonNames,
		}
		supervisor.Start()
		return
	}

	// ========================================================================
	// Daemon routine - launch all daemons at once.
	// ========================================================================
	// Prepare environmental changes
	if gomaxprocs > 0 {
		oldGomaxprocs := runtime.GOMAXPROCS(gomaxprocs)
		logger.Warning("main", "", nil, "GOMAXPROCS has been changed from %d to %d", oldGomaxprocs, gomaxprocs)
	} else {
		logger.Warning("main", "", nil, "GOMAXPROCS is unchanged at %d", runtime.GOMAXPROCS(0))
	}
	if disableConflicts {
		DisableConflicts()
	}

	daemonErrs := make(chan error, len(daemonNames))
	for _, daemonName := range daemonNames {
		// Daemons are started asynchronously because the order of startup does not matter.
		switch daemonName {
		case launcher.DNSDName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetDNSD().StartAndBlock())
			}(daemonName)
		case launcher.HTTPDName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetHTTPD().StartAndBlockWithTLS())
			}(daemonName)
		case launcher.InsecureHTTPDName:
			go func(daemonName string) {
				/*
					There is not an independent port settings for launching both TLS-enabled and TLS-free HTTP servers
					at the same time. If user really wishes to launch both at the same time, the TLS-free HTTP server
					will fallback to use port number 80.
				*/
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetHTTPD().StartAndBlockNoTLS(80))
			}(daemonName)
		case launcher.MaintenanceName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetMaintenance().StartAndBlock())
			}(daemonName)
		case launcher.PlainSocketName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetPlainSocketDaemon().StartAndBlock())
			}(daemonName)
		case launcher.SerialPortDaemonName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetSerialPortDaemon().StartAndBlock())
			}(daemonName)
		case launcher.SimpleIPSvcName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetSimpleIPSvcD().StartAndBlock())
			}(daemonName)
		case launcher.SMTPDName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetMailDaemon().StartAndBlock())
			}(daemonName)
		case launcher.SNMPDName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetSNMPD().StartAndBlock())
			}(daemonName)
		case launcher.SOCKDName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetSockDaemon().StartAndBlock())
			}(daemonName)
		case launcher.TelegramName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetTelegramBot().StartAndBlock())
			}(daemonName)
		case launcher.AutoUnlockName:
			go func(daemonName string) {
				daemonErrs <- terminatedDaemonToError(daemonName, config.GetAutoUnlock().StartAndBlock())
			}(daemonName)
		}
	}

	if benchmark {
		// Wait a short while for daemons to settle, then run benchmark in the background.
		logger.Info("main", "", nil, "benchmark is about to commence in 60 seconds")
		time.Sleep(60 * time.Second)
		bench := launcher.Benchmark{
			Config:      &config,
			DaemonNames: daemonNames,
			Logger:      logger,
			HTTPPort:    ProfilerHTTPPort,
		}
		go bench.RunBenchmarkAndProfiler()
	}

	// Wait for daemons to quit (they really should not).
	for i := 0; i < len(daemonNames); i++ {
		err := <-daemonErrs
		logger.Warning("main", "", err, "a daemon has aborted")
	}
	logger.Abort("main", "", nil, "all daemons have failed to start, laitos will now exit.")
}
