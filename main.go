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

var logger = misc.Logger{ComponentName: "laitos", ComponentID: strconv.Itoa(os.Getpid())}

// ExtractEncryptedArchive reads password from standard input and extracts archive file into the directory.
func ExtractEncryptedArchive(destDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter password to decrypt archive:")
	password, _, err := reader.ReadLine()
	if err != nil {
		misc.DefaultLogger.Fatalf("ExtractEncryptedArchive", "main", err, "failed to read password")
		return
	}
	/*
		This time, the temp file does not have to live in a ramdisk, because the extracted content does not have to be
		in the memory anyways.
	*/
	tmpFile, err := ioutil.TempFile("", "laitos-launcher-utility-extract")
	if err != nil {
		misc.DefaultLogger.Fatalf("ExtractEncryptedArchive", "main", err, "failed to create temporary file")
		return
	}
	tmpFile.Close()
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			misc.DefaultLogger.Printf("ExtractEncryptedArchive", "main", err, "failed to delete temporary file")
		}
	}()
	password = []byte(strings.TrimSpace(string(password)))
	fmt.Println("Result is (nil means success): ", encarchive.Extract(archivePath, tmpFile.Name(), destDir, password))
}

// MakeEncryptedArchive reads password from standard input and uses it to encrypt and archive the directory.
func MakeEncryptedArchive(srcDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter a password to encrypt the archive:")
	password, _, err := reader.ReadLine()
	if err != nil {
		misc.DefaultLogger.Fatalf("ExtractEncryptedArchive", "main", err, "failed to read password")
		return
	}
	password = []byte(strings.TrimSpace(string(password)))
	fmt.Println("Result is (nil means success): ", encarchive.Archive(srcDir, archivePath, password))
}

// StartPasswordWebServer starts the password input web server.
func StartPasswordWebServer(port int, url, archivePath string) {
	ws := passwdserver.WebServer{
		Port:            port,
		URL:             url,
		ArchiveFilePath: archivePath,
	}
	if err := ws.Start(); err != nil {
		logger.Fatalf("StartPasswordWebServer", "main", err, "failed to start the web server")
		return
	}
	// Wait almost indefinitely (~5 years) because this is the main routine of this CLI action
	time.Sleep(5 * 365 * 24 * time.Hour)
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
	var disableConflicts, tuneSystem, debug, swapOff bool
	var gomaxprocs int
	flag.StringVar(&misc.ConfigFilePath, launcher.ConfigFlagName, "", "(Mandatory) path to configuration file in JSON syntax")
	flag.StringVar(&daemonList, launcher.DaemonsFlagName, "", "(Mandatory) comma-separated daemons to start (dnsd, httpd, insecurehttpd, maintenance, plainsocket, smtpd, telegram)")
	flag.BoolVar(&disableConflicts, "disableconflicts", false, "(Optional) automatically stop and disable other daemon programs that may cause port usage conflicts")
	flag.BoolVar(&swapOff, "swapoff", false, "(Optional) turn off all swap files and partitions for improved system security")
	flag.BoolVar(&tuneSystem, "tunesystem", false, "(Optional) tune operating system parameters for optimal performance")
	flag.BoolVar(&debug, "debug", false, "(Optional) print goroutine stack traces upon receiving interrupt signal")
	flag.IntVar(&gomaxprocs, "gomaxprocs", 0, "(Optional) set gomaxprocs")
	// Process launcher flags
	var sl bool
	var slPort int
	var slArchivePath string
	var slURL string
	flag.BoolVar(&sl, passwdserver.CLIFlag, false, "(Optional) trigger \"special-launch\"")
	flag.IntVar(&slPort, "slport", 80, "(Optional) special-launch: port number")
	flag.StringVar(&slArchivePath, "slarchive", "", "(Optional) special-launch: archive path")
	flag.StringVar(&slURL, "slurl", "", "(Optional) special-launch: url that must include prefix slash")
	// Process launcher utility mode flags
	var slu, sluDir, sluFile string
	flag.StringVar(&slu, "slu", "", "(Optional) special-launch: utility name (extract, archive)")
	flag.StringVar(&sluDir, "sludir", "", "(Optional) special-launch: source/target directory")
	flag.StringVar(&sluFile, "slufile", "", "(Optional) special-launch: source/target archive file")
	// Internal supervisor flag
	var isSupervisor bool = true
	flag.BoolVar(&isSupervisor, launcher.SupervisorFlagName, true, "(Internal use only) enter supervisor mode")

	flag.Parse()

	// ========================================================================
	// Utility mode - encrypted data launcher utilities do not run daemons.
	// ========================================================================
	if slu != "" {
		switch slu {
		case "extract":
			ExtractEncryptedArchive(sluDir, sluFile)
		case "archive":
			MakeEncryptedArchive(sluDir, sluFile)
		default:
			logger.Fatalf("main", "", nil, "please provide mode of operation (extract|archive) for parameter slu")
		}
		return
	}

	// ========================================================================
	// Encrypted data launcher mode - launch the password input web server.
	// ========================================================================
	if sl {
		StartPasswordWebServer(slPort, slURL, slArchivePath)
		return
	}
	/*
		Encrypted data launcher (if any) has finished its task to decrypt program data, now it is time to launch supervisor
		which then launches daemons.
	*/

	// ========================================================================
	// Prepare configuration for supervisor mode or daemon mode.
	// ========================================================================
	// Parse configuration JSON file
	if misc.ConfigFilePath == "" {
		logger.Fatalf("main", "", nil, "please provide a configuration file (-config)")
		return
	}
	var err error
	misc.ConfigFilePath, err = filepath.Abs(misc.ConfigFilePath)
	if err != nil {
		logger.Fatalf("main", "", err, "failed to determine absolute path of config file \"%s\"", misc.ConfigFilePath)
	}
	var config launcher.Config
	configBytes, err := ioutil.ReadFile(misc.ConfigFilePath)
	if err != nil {
		logger.Fatalf("main", "", err, "failed to read config file \"%s\"", misc.ConfigFilePath)
		return
	}
	if err := config.DeserialiseFromJSON(configBytes); err != nil {
		logger.Fatalf("main", "", err, "failed to deserialise config file \"%s\"", misc.ConfigFilePath)
		return
	}
	// Figure out what daemons are to be started
	daemonNames := regexp.MustCompile(`\w+`).FindAllString(daemonList, -1)
	if daemonNames == nil || len(daemonNames) == 0 {
		logger.Fatalf("main", "", nil, "please provide comma-separated list of daemon services to start (-daemons).")
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
			logger.Fatalf("main", "", err, "unknown daemon name \"%s\"", daemonName)
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
		logger.Warningf("main", "", nil, "GOMAXPROCS has been changed from %d to %d", oldGomaxprocs, gomaxprocs)
	} else {
		logger.Warningf("main", "", nil, "GOMAXPROCS is unchanged at %d", runtime.GOMAXPROCS(0))
	}
	if debug {
		DumpGoroutinesOnInterrupt()
	}
	if disableConflicts {
		DisableConflictingDaemons()
	}
	if swapOff {
		SwapOff()
	}
	if tuneSystem {
		logger.Warningf("main", "", nil, "System tuning result is: \n%s", toolbox.TuneLinux())
	}
	ReseedPseudoRand()
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
				daemonErrs <- config.GetHTTPD().StartAndBlock()
			}()
		case launcher.InsecureHTTPDName:
			go func() {
				daemonErrs <- config.GetInsecureHTTPD().StartAndBlock()
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
	for i := 0; i < len(daemonNames); i++ {
		err := <-daemonErrs
		logger.Warningf("main", "", err, "a daemon has encountered and failed to start")
	}
	logger.Fatalf("main", "", nil, "all daemons have failed to start, laitos will now exit.")
}
