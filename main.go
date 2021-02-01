/*
laitos web server suite offers the simplest way to host your personal website, receive Emails, block ads and malicious websites with a DNS server, and much more!

And for geeks 🤓 - as a professional geek, you need Internet access whenever and wherever!

laitos connects to primitive infrastructure such as telephone, SMS, and satellite terminal network to offer reliable access to many Internet features, such as:

- Browse news, weather, and Twitter.
- Keep in touch via Email, telephone call, and SMS.
- Browse the web via a text-based JavaScript-capable browser.
- Run Linux/Windows shell commands.
- Generate 2nd factor authentication code.
- ... more apps to explore!
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/hzgl"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/lambda"
	"github.com/HouzuoGuo/laitos/launcher"
	"github.com/HouzuoGuo/laitos/launcher/passwdserver"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
	"github.com/aws/aws-xray-sdk-go/awsplugins/beanstalk"
	"github.com/aws/aws-xray-sdk-go/awsplugins/ec2"
	"github.com/aws/aws-xray-sdk-go/awsplugins/ecs"
	"github.com/aws/aws-xray-sdk-go/strategy/ctxmissing"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/aws/aws-xray-sdk-go/xraylog"
)

var (
	// pprofHTTPPort is the localhost port to listen on for serving pprof profiling data over HTTP.
	// The port number must differ from those used with regular HTTP and HTTPS servers.
	pprofHTTPPort int
	logger        = lalog.Logger{ComponentName: "main", ComponentID: []lalog.LoggerIDField{{Key: "PID", Value: os.Getpid()}}}
)

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
	content, err := misc.Decrypt(filePath, strings.TrimSpace(string(password)))
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
	if err := misc.Encrypt(filePath, password); err != nil {
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
		version, which means the web server often runs into port conflicts when its updated version starts up.
		AutoRestart function helps to restart the server.
	*/
	AutoRestart(logger, "StartPasswordWebServer", ws.Start)
	// Wait indefinitely upon success, because this function is the main function of the web server.
	select {}
}

/*
main runs one of several distinct routines according to the presented combination of command line flags:

- Maintain encrypted program data files: -datautil=encrypt|decrypt

- Launch a simple web server to let user enter program data decryption password, and then proceeds to launch laitos with supervisor:
  -pwdserver -pwdserverport=12345 -pwdserverurl=/my-password-input-page
	This routine is useful when some program data files such as configuration JSON or TLS certificate key are encrypted.

- Launch an AWS Lambda handler that proxies HTTP requests to laitos web server: -awslambda=true
	This routine handles the requests in an independent goroutine, it is compatible with supervisor but incompatible with "-pwdserver".

- Launch a supervisor that automatically restarts laitos main process in case of crash: -supervisor=true (already true by default)
  This is the routine of choice for launching laitos as an OS daemon service.

- Launch all specified daemons: -config c.json -daemons httpd,smtpd... -supervisor=false
  Supervisor launches laitos main process this way.

- Launch a benchmark routine that feeds random input to (nearly) all started daemons: -benchmark=true
  This routine is occasionally used for fuzzy-test daemons.

*/
func main() {
	hzgl.HZGL()
	// Process command line flags
	var daemonList string
	var disableConflicts, debug, awsLambda bool
	var gomaxprocs int
	flag.StringVar(&misc.ConfigFilePath, launcher.ConfigFlagName, "", "(Mandatory) path to configuration file in JSON syntax")
	flag.StringVar(&daemonList, launcher.DaemonsFlagName, "", "(Mandatory) comma-separated daemons to start (autounlock, dnsd, httpd, insecurehttpd, maintenance, plainsocket, serialport, simpleipsvcd, smtpd, snmpd, sockd, telegram)")
	flag.BoolVar(&disableConflicts, "disableconflicts", false, "(Optional) automatically stop and disable other daemon programs that may cause port usage conflicts")
	flag.BoolVar(&awsLambda, launcher.LambdaFlagName, false, "(Optional) run AWS Lambda handler to proxy HTTP requests to laitos web server")
	flag.BoolVar(&misc.EnableAWSIntegration, "awsinteg", false, "(Optional) activate all points of integration with various AWS services such as sending warning log entries to SQS")
	flag.BoolVar(&debug, "debug", false, "(Optional) print goroutine stack traces upon receiving interrupt signal")
	flag.IntVar(&pprofHTTPPort, "profhttpport", pprofHTTPPort, "(Optional) serve program profiling data (pprof) over HTTP on this port at localhost")
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
	flag.BoolVar(&isSupervisor, launcher.SupervisorFlagName, true, "(Internal use only) launch a supervisor process to auto-restart laitos main process in case of crash")

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
	// AWS lambda handler starts an independent goroutine to proxy HTTP requests to laitos web server.
	// ========================================================================
	if awsLambda {
		// Use environment variable PORT to tell HTTP (not HTTPS) server to listen on port expected by lambda handler
		os.Setenv("PORT", strconv.Itoa(lambda.UpstreamWebServerPort))
		// Unfortunately without encrypting program config file it is impossible to set LAITOS_HTTP_URL_ROUTE_PREFIX
		handler := &lambda.Handler{}
		go handler.StartAndBlock()
		// Proceed to launch the daemons, including the HTTP web server that lambda handler forwards incoming requess to.
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
		Here comes the preparation for both supervisor and daemons:
	*/
	// Read unencrypted configuration data from environment variable, or possibly encrypted configuration from JSON file.
	configBytes := []byte(strings.TrimSpace(os.Getenv("LAITOS_CONFIG")))
	if len(configBytes) == 0 {
		// Proceed to read the config file
		if misc.ConfigFilePath == "" {
			logger.Abort("main", "config", nil, "please provide a configuration file (-config)")
			return
		}
		var err error
		misc.ConfigFilePath, err = filepath.Abs(misc.ConfigFilePath)
		if err != nil {
			logger.Abort("main", "config", err, "failed to determine absolute path of config file \"%s\"", misc.ConfigFilePath)
			return
		}
		// If config file is encrypted, read its password from standard input.
		var isEncrypted bool
		configBytes, isEncrypted, err = misc.IsEncrypted(misc.ConfigFilePath)
		if err != nil {
			logger.Abort("main", "config", err, "failed to read configuration file \"%s\"", misc.ConfigFilePath)
			return
		}
		if isEncrypted {
			logger.Info("main", "config", nil, "the configuration file is encrypted, please pipe or type decryption password followed by Enter (new-line).")
			go func() {
				// Collect program data decryption password from STDIN
				pwdReader := bufio.NewReader(os.Stdin)
				pwdFromStdin, err := pwdReader.ReadString('\n')
				if err == nil {
					misc.ProgramDataDecryptionPasswordInput <- strings.TrimSpace(pwdFromStdin)
				} else {
					logger.Warning("main", "config", err, "failed to read decryption password from STDIN")
				}
			}()
			// AWS lambda handler may also supply this password
			pwd := <-misc.ProgramDataDecryptionPasswordInput
			misc.ProgramDataDecryptionPassword = pwd
			if configBytes, err = misc.Decrypt(misc.ConfigFilePath, misc.ProgramDataDecryptionPassword); err != nil {
				logger.Abort("main", "config", err, "failed to decrypt config file")
				return
			}
		}
	} else {
		logger.Info("main", "", nil, "reading %d bytes of JSON configuration from environment variable LAITOS_CONFIG", len(configBytes))
	}

	var config launcher.Config
	if err := config.DeserialiseFromJSON(configBytes); err != nil {
		logger.Abort("main", "", err, "failed to deserialise/initialise program configuration")
		return
	}
	// Figure out what daemons are to be started
	daemonNames := regexp.MustCompile(`\w+`).FindAllString(daemonList, -1)
	if len(daemonNames) == 0 {
		time.Sleep(1000 * time.Second)
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
			logger.Abort("main", "", nil, "unrecognised daemon name \"%s\"", daemonName)
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
	// From this point and onward, the code enjoys the supervision and safety provided by the supervisor.

	if misc.EnableAWSIntegration && inet.IsAWS() {
		// Integrate the decorated handler with AWS x-ray. The crucial x-ray daemon program seems to be only capable of running on AWS compute resources.
		os.Setenv("AWS_XRAY_CONTEXT_MISSING", "LOG_ERROR")
		beanstalk.Init()
		ecs.Init()
		ec2.Init()
		_ = xray.Configure(xray.Config{ContextMissingStrategy: ctxmissing.NewDefaultIgnoreErrorStrategy()})
		xray.SetLogger(xraylog.NewDefaultLogger(os.Stderr, xraylog.LogLevelWarn))
	}

	// ========================================================================
	// Daemon routine - launch all daemons at once.
	// ========================================================================

	// Prepare environmental changes
	if gomaxprocs > 0 {
		oldGomaxprocs := runtime.GOMAXPROCS(gomaxprocs)
		logger.Warning("main", "gomaxprocs", nil, "GOMAXPROCS has been changed from %d to %d", oldGomaxprocs, gomaxprocs)
	} else {
		logger.Warning("main", "gomaxprocs", nil, "GOMAXPROCS is unchanged at %d", runtime.GOMAXPROCS(0))
	}
	if disableConflicts {
		DisableConflicts()
	}
	CopyNonEssentialUtilitiesInBackground()
	InstallOptionalLoggerSQSCallback(config.AWSIntegration.SendWarningLogToSQSURL)

	for _, daemonName := range daemonNames {
		// Daemons are started asynchronously and the order does not matter
		switch daemonName {
		case launcher.DNSDName:
			go AutoRestart(logger, daemonName, config.GetDNSD().StartAndBlock)
		case launcher.HTTPDName:
			go AutoRestart(logger, daemonName, config.GetHTTPD().StartAndBlockWithTLS)
		case launcher.InsecureHTTPDName:
			/*
				There is not an independent port settings for launching both TLS-enabled and TLS-free HTTP servers
				at the same time. If user really wishes to launch both at the same time, the TLS-free HTTP server
				will fallback to use port number 80.
			*/
			go AutoRestart(logger, daemonName, func() error {
				return config.GetHTTPD().StartAndBlockNoTLS(80)
			})
		case launcher.MaintenanceName:
			go AutoRestart(logger, daemonName, config.GetMaintenance().StartAndBlock)
		case launcher.PhoneHomeName:
			go AutoRestart(logger, daemonName, config.GetPhoneHomeDaemon().StartAndBlock)
		case launcher.PlainSocketName:
			go AutoRestart(logger, daemonName, config.GetPlainSocketDaemon().StartAndBlock)
		case launcher.SerialPortDaemonName:
			go AutoRestart(logger, daemonName, config.GetSerialPortDaemon().StartAndBlock)
		case launcher.SimpleIPSvcName:
			go AutoRestart(logger, daemonName, config.GetSimpleIPSvcD().StartAndBlock)
		case launcher.SMTPDName:
			go AutoRestart(logger, daemonName, config.GetMailDaemon().StartAndBlock)
		case launcher.SNMPDName:
			go AutoRestart(logger, daemonName, config.GetSNMPD().StartAndBlock)
		case launcher.SOCKDName:
			go AutoRestart(logger, daemonName, config.GetSockDaemon().StartAndBlock)
		case launcher.TelegramName:
			go AutoRestart(logger, daemonName, config.GetTelegramBot().StartAndBlock)
		case launcher.AutoUnlockName:
			go AutoRestart(logger, daemonName, config.GetAutoUnlock().StartAndBlock)
		}
	}

	// Start an HTTP server on localhost to serve program profiling data
	if pprofHTTPPort > 0 {
		// Expose the entire selection of profiling profiles identical to the ones installed by pprof standard library package
		pprofMux := http.NewServeMux()
		pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
		pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		logger.Info("main", "pprof", nil, "serving program profiling data over HTTP server on port %d", pprofHTTPPort)
		if err := http.ListenAndServe(net.JoinHostPort("localhost", strconv.Itoa(pprofHTTPPort)), pprofMux); err != nil {
			logger.Warning("main", "pprof", err, "failed to start HTTP server for program profiling data")
		}
	}

	// Daemons are already started in background goroutines, the main function now waits indefinitely.
	select {}
}
