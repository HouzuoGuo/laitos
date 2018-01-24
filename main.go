package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/HouzuoGuo/laitos/launcher"
	"github.com/HouzuoGuo/laitos/launcher/encarchive"
	"github.com/HouzuoGuo/laitos/launcher/passwdserver"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	ProfilerHTTPPort = 19151 // ProfilerHTTPPort is to be listened by net/http/pprof HTTP server when benchmark is turned on
)

var logger = misc.Logger{ComponentName: "laitos", ComponentID: strconv.Itoa(os.Getpid())}

// ExtractEncryptedArchive reads password from standard input and extracts archive file into the directory.
func ExtractEncryptedArchive(destDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter password to decrypt archive (no echo):")
	SetTermEcho(false)
	password, _, err := reader.ReadLine()
	SetTermEcho(true)
	if err != nil {
		misc.DefaultLogger.Abort("ExtractEncryptedArchive", "main", err, "failed to read password")
		return
	}
	/*
		This time, the temp file does not have to live in a ramdisk, because the extracted content does not have to be
		in the memory anyways.
	*/
	tmpFile, err := ioutil.TempFile("", "laitos-launcher-utility-extract")
	if err != nil {
		misc.DefaultLogger.Abort("ExtractEncryptedArchive", "main", err, "failed to create temporary file")
		return
	}
	tmpFile.Close()
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil && !os.IsNotExist(err) {
			misc.DefaultLogger.Info("ExtractEncryptedArchive", "main", err, "failed to delete temporary file")
		}
	}()
	password = []byte(strings.TrimSpace(string(password)))
	err = encarchive.Extract(archivePath, tmpFile.Name(), destDir, password)
	if err == nil {
		fmt.Println("Success")
	} else {
		fmt.Println("Error: ", err.Error())
	}
}

// MakeEncryptedArchive reads password from standard input and uses it to encrypt and archive the directory.
func MakeEncryptedArchive(srcDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter a password to encrypt the archive (no echo):")
	SetTermEcho(false)
	password, _, err := reader.ReadLine()
	SetTermEcho(true)
	if err != nil {
		misc.DefaultLogger.Abort("ExtractEncryptedArchive", "main", err, "failed to read password")
		return
	}
	password = []byte(strings.TrimSpace(string(password)))
	err = encarchive.Archive(srcDir, archivePath, password)
	if err == nil {
		fmt.Println("Success")
	} else {
		fmt.Println("Error: ", err.Error())
	}
}

// StartPasswordWebServer starts the password input web server.
func StartPasswordWebServer(port int, url, archivePath string) {
	ws := passwdserver.WebServer{
		Port:            port,
		URL:             url,
		ArchiveFilePath: archivePath,
	}
	/*
		On Amazon ElasitcBeanstalk, application update cannot reliably kill the old program prior to launching the new
		version, which means the web server often runs into port conflicts. Therefore, make at most 10 attempts at
		starting the web server.
	*/
	for i := 0; i < 10; i++ {
		if err := ws.Start(); err == nil {
			// Upon success, wait almost indefinitely (~5 years) because this is the main routine of this CLI action.
			time.Sleep(5 * 365 * 24 * time.Hour)
		} else {
			// Retry upon failure
			logger.Info("StartPasswordWebServer", "main", err, "failed to start the web server (attempt %d)", i)
			time.Sleep(5 * time.Second)
		}
	}
	logger.Abort("StartPasswordWebServer", "main", nil, "failed to start the web server after many attempts")
	return

}

/*
main runs one of several modes, as dictated by input command line flags:

- Utilities for maintaining encrypted data archive (-slu extract|archive)

- (Optional) encrypted data launcher (-sl) will eventually start the supervisor (-supervisor=true)

- Supervisor (-supervisor=true) is responsible for forking main process to launch daemons:

- The forked main process runs with flag -supervisor=false
*/
func main() {
	// Process command line flags
	var daemonList string
	var disableConflicts, tuneSystem, debug, swapOff, benchmark bool
	var gomaxprocs int
	flag.StringVar(&misc.ConfigFilePath, launcher.ConfigFlagName, "", "(Mandatory) path to configuration file in JSON syntax")
	flag.StringVar(&daemonList, launcher.DaemonsFlagName, "", "(Mandatory) comma-separated daemons to start (dnsd, httpd, insecurehttpd, maintenance, plainsocket, smtpd, telegram)")
	flag.BoolVar(&disableConflicts, "disableconflicts", false, "(Optional) automatically stop and disable other daemon programs that may cause port usage conflicts")
	flag.BoolVar(&swapOff, "swapoff", false, "(Optional) turn off all swap files and partitions for improved system security")
	flag.BoolVar(&tuneSystem, "tunesystem", false, "(Optional) tune operating system parameters for optimal performance")
	flag.BoolVar(&debug, "debug", false, "(Optional) print goroutine stack traces upon receiving interrupt signal")
	flag.BoolVar(&benchmark, "benchmark", false, fmt.Sprintf("(Optional) continuously run benchmark routines on active daemons while exposing net/http/pprof on port %d", ProfilerHTTPPort))
	flag.IntVar(&gomaxprocs, "gomaxprocs", 0, "(Optional) set gomaxprocs")
	// Encrypted data archive launcher (password input server) flags
	var pwdServer bool
	var pwdServerPort int
	var pwdServerData string
	var pwdServerURL string
	flag.BoolVar(&pwdServer, passwdserver.CLIFlag, false, "(Optional) launch web server to accept password for decrypting encrypted program data")
	flag.IntVar(&pwdServerPort, passwdserver.CLIFlag+"port", 80, "(Optional) port number of the password web server")
	flag.StringVar(&pwdServerData, passwdserver.CLIFlag+"data", "", "(Optional) location of encrypted program data archive")
	flag.StringVar(&pwdServerURL, passwdserver.CLIFlag+"url", "", "(Optional) password input URL")
	// Encrypted data archive utility flags
	var dataUtil, dataUtilDir, dataUtilFile string
	flag.StringVar(&dataUtil, "datautil", "", "(Optional) program data encryption utility: extract|archive")
	flag.StringVar(&dataUtilDir, "datautildir", "", "(Optional) program data encryption utility: extract destination or archive source directory")
	flag.StringVar(&dataUtilFile, "datautilfile", "", "(Optional) program data encryption utility: extract from or archive file location")
	// Internal supervisor flag
	var isSupervisor = true
	flag.BoolVar(&isSupervisor, launcher.SupervisorFlagName, true, "(Internal use only) enter supervisor mode")

	flag.Parse()

	// ========================================================================
	// Utility mode - Encrypted data archive utilities do not run daemons.
	// ========================================================================
	if dataUtil != "" {
		switch dataUtil {
		case "extract":
			ExtractEncryptedArchive(dataUtilDir, dataUtilFile)
		case "archive":
			MakeEncryptedArchive(dataUtilDir, dataUtilFile)
		default:
			logger.Abort("main", "", nil, "please provide mode of operation (extract|archive) for parameter \"-datautil\"")
		}
		return
	}

	// ========================================================================
	// Encrypted data archive launcher mode - launch the password input web server.
	// ========================================================================
	if pwdServer {
		StartPasswordWebServer(pwdServerPort, pwdServerURL, pwdServerData)
		return
	}
	/*
		Encrypted data launcher (if any) has finished its task to decrypt program data, now it is time to launch supervisor
		which then launches daemons.
	*/

	// ========================================================================
	// Prepare configuration for supervisor mode or daemon mode.
	// ========================================================================
	LockMemory()
	// Parse configuration JSON file
	if misc.ConfigFilePath == "" {
		logger.Abort("main", "", nil, "please provide a configuration file (-config)")
		return
	}
	var err error
	misc.ConfigFilePath, err = filepath.Abs(misc.ConfigFilePath)
	if err != nil {
		logger.Abort("main", "", err, "failed to determine absolute path of config file \"%s\"", misc.ConfigFilePath)
	}
	var config launcher.Config
	configBytes, err := ioutil.ReadFile(misc.ConfigFilePath)
	if err != nil {
		logger.Abort("main", "", err, "failed to read config file \"%s\"", misc.ConfigFilePath)
		return
	}
	if err := config.DeserialiseFromJSON(configBytes); err != nil {
		logger.Abort("main", "", err, "failed to deserialise config file \"%s\"", misc.ConfigFilePath)
		return
	}
	// Figure out what daemons are to be started
	daemonNames := regexp.MustCompile(`\w+`).FindAllString(daemonList, -1)
	if daemonNames == nil || len(daemonNames) == 0 {
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
	// Supervisor mode - fork a main process to run daemons.
	// This mode flag is turned on by default so that when laitos daemons are
	// protected by supervisor by default.
	// ========================================================================
	if isSupervisor {
		supervisor := &launcher.Supervisor{CLIFlags: os.Args[1:], Config: config, DaemonNames: daemonNames}
		supervisor.Start()
		return
	}

	// ========================================================================
	// Daemon mode - launch all daemons at once.
	// This is the mode launched by supervisor in a forked process.
	// ========================================================================
	// Prepare some environmental changes
	if gomaxprocs > 0 {
		oldGomaxprocs := runtime.GOMAXPROCS(gomaxprocs)
		logger.Warning("main", "", nil, "GOMAXPROCS has been changed from %d to %d", oldGomaxprocs, gomaxprocs)
	} else {
		logger.Warning("main", "", nil, "GOMAXPROCS is unchanged at %d", runtime.GOMAXPROCS(0))
	}
	if debug {
		DumpGoroutinesOnInterrupt()
	}
	if disableConflicts {
		DisableConflicts()
	}
	if swapOff {
		SwapOff()
	}
	if tuneSystem {
		logger.Warning("main", "", nil, "System tuning result is: \n%s", toolbox.TuneLinux())
	}
	ReseedPseudoRand()

	// Prepare utility programs that are not essential but helpful to certain toolbox features and daemons
	misc.PrepareUtilities(logger)
	daemonErrs := make(chan error, len(daemonNames))
	for _, daemonName := range daemonNames {
		// Daemons are started asynchronously, the order of startup does not matter.
		switch daemonName {
		case launcher.DNSDName:
			go func() {
				daemonErrs <- config.GetDNSD().StartAndBlock()
			}()
		case launcher.HTTPDName:
			go func() {
				daemonErrs <- config.GetHTTPD().StartAndBlockWithTLS()
			}()
		case launcher.InsecureHTTPDName:
			go func() {
				daemonErrs <- config.GetHTTPD().StartAndBlockNoTLS(80)
			}()
		case launcher.MaintenanceName:
			go func() {
				daemonErrs <- config.GetMaintenance().StartAndBlock()
			}()
		case launcher.PlainSocketName:
			go func() {
				daemonErrs <- config.GetPlainSocketDaemon().StartAndBlock()
			}()
		case launcher.SMTPDName:
			go func() {
				daemonErrs <- config.GetMailDaemon().StartAndBlock()
			}()
		case launcher.SOCKDName:
			go func() {
				daemonErrs <- config.GetSockDaemon().StartAndBlock()
			}()
		case launcher.TelegramName:
			go func() {
				daemonErrs <- config.GetTelegramBot().StartAndBlock()
			}()
		}
	}

	// ========================================================================
	// Daemon mode - optionally run benchmark in the background.
	// ========================================================================
	if benchmark {
		logger.Info("main", "", nil, "benchmark is about to commence in 30 seconds")
		time.Sleep(30 * time.Second)
		bench := launcher.Benchmark{
			Config:      &config,
			DaemonNames: daemonNames,
			Logger:      logger,
			HTTPPort:    ProfilerHTTPPort,
		}
		go bench.RunBenchmarkAndProfiler()
	}

	// ========================================================================
	// Daemon mode - wait for daemons to crash (ha ha ha).
	// ========================================================================
	for i := 0; i < len(daemonNames); i++ {
		err := <-daemonErrs
		logger.Warning("main", "", err, "a daemon has encountered an error and failed to start")
	}
	logger.Abort("main", "", nil, "all daemons have failed to start, laitos will now exit.")
}
