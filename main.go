/*
**laitos** software suite offers all you need for hosting a personal website,
receiving Emails, blocking ads with a DNS server.

And now for the geeks 🤓 - as a professional geek, you need Internet access
whenever and wherever! laitos inter-operates with
[telephone, SMS](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook),
[satellite terminals](https://github.com/HouzuoGuo/laitos/wiki/Tips-for-using-apps-over-satellite),
and
[DNS](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server#invoke-app-commands-via-dns-queries),
to give you access to Internet features such as:

-   Browse news, weather, and Twitter.
-   Keep in touch via Email, telephone call, and SMS.
-   Browse the web via a text-based JavaScript-capable browser.
-   Run Linux/Windows shell commands.
-   Generate 2nd factor authentication code.
-   ... more apps to explore!

Check out the
[comprehensive component list](https://github.com/HouzuoGuo/laitos/wiki/Component-list)
to explore all of the possibilities!
*/
package main

import (
	"bufio"
	"context"
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/HouzuoGuo/laitos/daemon/httpd"
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

const (
	// AppEngineDataDir is the relative path to a data directory that contains config files and data files required for launching laitos program
	// on GCP app engine.
	AppEngineDataDir = "./gcp_appengine_data"
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
	platform.SetTermEcho(false)
	defer platform.SetTermEcho(true)
	reader := bufio.NewReader(os.Stdin)
	lalog.DefaultLogger.Info("DecryptFile", "", nil, "Please enter a password to decrypt file \"%s\" (terminal won't echo):\n", filePath)
	password, err := reader.ReadString('\n')
	if err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to read password")
		return
	}
	content, err := misc.Decrypt(filePath, strings.TrimSpace(password))
	if err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to decrypt file")
		return
	}
	if err := ioutil.WriteFile(filePath, content, 0600); err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to decrypt file")
		return
	}
	lalog.DefaultLogger.Info("DecryptFile", "main", nil, "the file has been decrypted in-place")
}

/*
EncryptFile is a distinct routine of laitos main program, it reads password from standard input and uses it to encrypt
the input file in-place.
*/
func EncryptFile(filePath string) {
	platform.SetTermEcho(false)
	defer platform.SetTermEcho(true)
	reader := bufio.NewReader(os.Stdin)
	lalog.DefaultLogger.Info("EncryptFile", "", nil, "please enter a password to encrypt the file \"%s\" (terminal won't echo):\n", filePath)
	password, err := reader.ReadString('\n')
	if err != nil {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "failed to read password")
		return
	}
	lalog.DefaultLogger.Info("EncryptFile", "", nil, "enter the same password again (terminal won't echo):")
	passwordAgain, err := reader.ReadString('\n')
	if err != nil {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "failed to read password")
		return
	}
	if password != passwordAgain {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "The two passwords must match")
		return
	}
	password = strings.TrimSpace(password)
	if err := misc.Encrypt(filePath, password); err != nil {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "failed to encrypt file")
		return
	}
	lalog.DefaultLogger.Info("EncryptFile", "", nil, "the file has been encrypted in-place with a password %d characters long", len(password))
}

/*
main runs one of several distinct routines according to the presented combination of command line flags:

- Maintain encrypted program data files: -datautil=encrypt|decrypt

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
	var daemonList, passwordUnlockServers string
	var disableConflicts, debug, awsLambda bool
	var gomaxprocs int
	flag.StringVar(&misc.ConfigFilePath, launcher.ConfigFlagName, "", "(Mandatory) path to configuration file in JSON syntax")
	flag.StringVar(&daemonList, launcher.DaemonsFlagName, "", "(Mandatory) comma-separated list of daemon names to start (autounlock, dnsd, httpd, httpproxy, insecurehttpd, maintenance, passwdrpc, phonehome, plainsocket, serialport, simpleipsvcd, smtpd, snmpd, sockd, telegram)")
	flag.BoolVar(&awsLambda, launcher.LambdaFlagName, false, "(Optional) run AWS Lambda handler to proxy HTTP requests to laitos web server")
	// Internal supervisor flag
	var isSupervisor = true
	flag.BoolVar(&isSupervisor, launcher.SupervisorFlagName, true, "(Internal use only) launch a supervisor process to auto-restart laitos main process in case of crash")
	// Auxiliary features
	flag.BoolVar(&disableConflicts, "disableconflicts", false, "(Optional) automatically stop and disable other daemon programs that may cause port usage conflicts")
	flag.StringVar(&passwordUnlockServers, "passwordunlockservers", "", "(Optional) comma-separated list of server:port combos that offer password unlocking service (daemon \"passwdrpc\") over gRPC")
	// Optional integration features
	flag.BoolVar(&misc.EnableAWSIntegration, "awsinteg", false, "(Optional) activate all points of integration with various AWS services such as sending warning log entries to SQS")
	flag.BoolVar(&misc.EnablePrometheusIntegration, "prominteg", false, "(Optional) activate all points of integration with Prometheus such as collecting performance metrics and serving them over HTTP")
	// Diagnosis features
	flag.BoolVar(&debug, "debug", false, "(Optional) print goroutine stack traces upon receiving interrupt signal")
	flag.IntVar(&pprofHTTPPort, "profhttpport", pprofHTTPPort, "(Optional) serve program profiling data (pprof) over HTTP on this port at localhost")
	flag.IntVar(&gomaxprocs, "gomaxprocs", 0, "(Optional) set gomaxprocs")
	// Decryption password collector (password input server) flags
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

	flag.Parse()

	logger.Info("main", "", nil, "program is starting, here is a summary of the runtime environment:\n%s", platform.GetProgramStatusSummary(false))
	// FIXME: TODO: this main function is way too long >:-|
	if os.Getenv("GAE_ENV") == "standard" {
		misc.EnablePrometheusIntegration = true
		// Change working directory to the data directory (if not done yet).
		// All program config files and data files are expected to reside in the data directory.
		cwd, err := os.Getwd()
		if err != nil {
			logger.Abort("main", "", err, "failed to determine current working directory")
		}
		if path.Base(cwd) != path.Base(AppEngineDataDir) {
			if err := os.Chdir(AppEngineDataDir); err != nil {
				logger.Abort("main", "", err, "failed to change directory to %s", AppEngineDataDir)
				return
			}
		}
		// Read the value of CLI parameter "-daemons" from a text file
		daemonListContent, err := ioutil.ReadFile("daemonList")
		if err != nil {
			logger.Abort("main", "", err, "failed to read daemonList")
			return
		}
		// Find program configuration data (encrypted or otherwise) in "config.json"
		misc.ConfigFilePath = "config.json"
		daemonList = string(daemonListContent)
	}

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
		_ = os.Setenv(httpd.EnvironmentPortNumber, strconv.Itoa(lambda.UpstreamWebServerPort))
		// Unfortunately without encrypting program config file it is impossible to set LAITOS_HTTP_URL_ROUTE_PREFIX
		handler := &lambda.Handler{}
		go handler.StartAndBlock()
		// Proceed to launch the daemons, including the HTTP web server that lambda handler forwards incoming request to.
	}

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
			// There are multiple ways to collect the decryption password
			passwdRPCContext, passwdRPCCancel := context.WithCancel(context.Background())
			passwordCollectionServer := passwdserver.WebServer{
				Port: pwdServerPort,
				URL:  pwdServerURL,
			}
			if password := strings.TrimSpace(os.Getenv(misc.EnvironmentDecryptionPassword)); password != "" {
				logger.Info("main", "config", nil, "got decryption password of %d characters from environment variable %s", len(password), misc.EnvironmentDecryptionPassword)
				misc.ProgramDataDecryptionPasswordInput <- password
			} else {
				go func() {
					// Collect program data decryption password from STDIN, there is not an explicit cancellation for the buffered read.
					stdinReader := bufio.NewReader(os.Stdin)
					pwdFromStdin, err := stdinReader.ReadString('\n')
					if err == nil {
						logger.Info("main", "config", nil, "got decryption password from stdin")
						misc.ProgramDataDecryptionPasswordInput <- strings.TrimSpace(pwdFromStdin)
					} else {
						logger.Warning("main", "config", err, "failed to read decryption password from STDIN")
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
				// The AWS lambda handler is also able to retrieve the password from API gateway stage configuration and place it into the channel
			}
			plainTextPassword := <-misc.ProgramDataDecryptionPasswordInput
			misc.ProgramDataDecryptionPassword = plainTextPassword
			// Explicitly stop background routines that may be still trying to obtain a decryption password
			passwdRPCCancel()
			passwordCollectionServer.Shutdown()
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
		_ = os.Setenv("AWS_XRAY_CONTEXT_MISSING", "LOG_ERROR")
		_ = xray.Configure(xray.Config{ContextMissingStrategy: ctxmissing.NewDefaultIgnoreErrorStrategy()})
		xray.SetLogger(xraylog.NewDefaultLogger(ioutil.Discard, xraylog.LogLevelWarn))
		go func() {
			// These functions of aws lib take their sweet time, don't let them block main's progress. It's OK to miss a couple of traces.
			beanstalk.Init()
			ecs.Init()
			ec2.Init()
		}()
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
		case launcher.PasswdRPCName:
			go AutoRestart(logger, daemonName, config.GetPasswdRPCDaemon().StartAndBlock)
		case launcher.HTTPProxyName:
			go AutoRestart(logger, daemonName, config.GetHTTPProxyDaemon().StartAndBlock)
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
			// This server is not expected to shutdown
			logger.Warning("main", "pprof", err, "failed to start HTTP server for program profiling data")
		}
	}

	// Daemons are already started in background goroutines, the main function now waits indefinitely.
	select {}
}
