// laitos software suite offers all you need for hosting a personal website,
// receiving Emails, blocking ads with a DNS server.
//
// And now for the geeks 🤓 - as a professional geek, you need Internet access
// whenever and wherever! laitos inter-operates with
// [telephone, SMS](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook),
// [satellite terminals](https://github.com/HouzuoGuo/laitos/wiki/Tips-for-using-apps-over-satellite),
// and
// [DNS](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server#invoke-app-commands-via-dns-queries),
// to give you access to Internet features such as:
//
// -   Browse news, weather, and Twitter.
// -   Keep in touch via Email, telephone call, and SMS.
// -   Remotely control computers in your laitos fleet.
// -   ... more apps to explore!
//
// Check out the
// [comprehensive component list](https://github.com/HouzuoGuo/laitos/wiki/Component-list)
// to explore all of the possibilities!
package main

import (
	"flag"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/HouzuoGuo/laitos/cli"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/hzgl"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/lambda"
	"github.com/HouzuoGuo/laitos/launcher"
	"github.com/HouzuoGuo/laitos/launcher/passwdserver"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

var (
	// pprofHTTPPort is the localhost port to listen on for serving pprof profiling data over HTTP.
	// The port number must differ from those used with regular HTTP and HTTPS servers.
	pprofHTTPPort int
	logger        = &lalog.Logger{ComponentName: "main", ComponentID: []lalog.LoggerIDField{{Key: "PID", Value: os.Getpid()}}}
)

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
	flag.StringVar(&daemonList, launcher.DaemonsFlagName, "", "(Mandatory) comma-separated list of daemon names to start (autounlock, dnsd, httpd, httpproxy, insecurehttpd, maintenance, passwdrpc, phonehome, plainsocket, simpleipsvcd, smtpd, snmpd, sockd, telegram)")
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
	// TCP-over-DNS proxy client flags.

	var proxyOpts cli.ProxyCLIOptions
	flag.IntVar(&proxyOpts.Port, "proxyport", 8080, "(TCP-over-DNS optional) override the port of the local HTTP(S) proxy server on 127.0.0.12")
	flag.BoolVar(&proxyOpts.Debug, "proxydebug", false, "(TCP-over-DNS optional) turn on debug logs")
	flag.BoolVar(&proxyOpts.EnableDNSRelay, "proxyenablednsrelay", false, "(TCP-over-DNS optional) start a recursive resolver on 127.0.0.12:53 to relay queries to laitos DNS server")
	flag.StringVar(&proxyOpts.RecursiveResolverAddress, "proxyresolver", "", `(TCP-over-DNS optional) local/public recursive DNS resolver address ("ip:port") or empty for auto detection`)
	flag.IntVar(&proxyOpts.MaxSegmentLength, "proxyseglen", 0, "(TCP-over-DNS optional) override max segment length")
	flag.StringVar(&proxyOpts.LaitosDNSName, "proxydnsname", "", "(TCP-over-DNS mandatory) the DNS name of laitos DNS server")
	flag.StringVar(&proxyOpts.AccessOTPSecret, "proxyotpsecret", "", "(TCP-over-DNS mandatory) authorise connection requests using this OTP secret")
	flag.BoolVar(&proxyOpts.EnableTXT, "proxyenabletxt", false, "(TCP-over-DNS optional) send TXT queries instead of CNAME queries for higher bandwidth")
	flag.IntVar(&proxyOpts.DownstreamSegmentLength, "proxydownstreamseglen", 0, "(TCP-over-DNS optional) responder (downstream) maximum segment length")

	flag.Parse()

	// Enable common diagnosis and security features.
	logger.Info(nil, nil, "program is starting, here is a summary of the runtime environment:\n%s", platform.GetProgramStatusSummary(false))
	platform.LockMemory()
	cli.ClearDedupBuffersInBackground()
	cli.ReseedPseudoRandAndInBackground(logger)
	cli.StartProfilingServer(logger, pprofHTTPPort)
	if debug {
		cli.DumpGoroutinesOnInterrupt()
	}

	// ========================================================================
	// Non-daemon utility routines - laitos configuration data encryption.
	// ========================================================================
	if dataUtil != "" {
		cli.HandleSecurityDataUtil(dataUtil, dataUtilFile, logger)
		return
	}

	// ========================================================================
	// Non-daemon utility routines - TCP-over-DNS client.
	// ========================================================================
	if proxyOpts.LaitosDNSName != "" {
		cli.HandleTCPOverDNSClient(logger, proxyOpts)
		return
	}

	// Manipulate the daemon list parameter if running on Google App Engine.
	if newDaemonList := cli.GAEDaemonList(logger); newDaemonList != "" {
		daemonList = newDaemonList
	}

	// ========================================================================
	// AWS lambda handler starts an independent goroutine to proxy HTTP requests
	// to laitos web server.
	// The handler also retrieves the decryption password for the program
	// configuration from API gateway stage configuration, if provided.
	// ========================================================================
	if awsLambda {
		// Use environment variable PORT to tell HTTP (not HTTPS) server to listen on port expected by lambda handler
		_ = os.Setenv(httpd.EnvironmentPortNumber, strconv.Itoa(lambda.UpstreamWebServerPort))
		// Unfortunately without encrypting program config file it is impossible to set LAITOS_HTTP_URL_ROUTE_PREFIX
		handler := &lambda.Handler{}
		handler.Initialise()
		go handler.StartAndBlock()
		// Proceed to launch the daemons, including the HTTP web server that lambda handler forwards incoming request to.
	}

	// Read unencrypted configuration data from environment variable, or possibly encrypted configuration from JSON file.
	var config launcher.Config
	if err := config.DeserialiseFromJSON(cli.GetConfig(logger, pwdServer, pwdServerPort, pwdServerURL, passwordUnlockServers)); err != nil {
		logger.Abort(nil, err, "failed to retrieve/deserialise program configuration")
		return
	}
	// Figure out which daemons to start, make sure the names are valid.
	daemonNames := regexp.MustCompile(`\w+`).FindAllString(daemonList, -1)
	if len(daemonNames) == 0 {
		logger.Abort(nil, nil, "please provide comma-separated list of daemon services to start (-daemons).")
		return
	}
	for _, daemonName := range daemonNames {
		var found bool
		for _, goodName := range launcher.AllDaemons {
			if daemonName == goodName {
				found = true
			}
		}
		if !found {
			logger.Abort(nil, nil, "unrecognised daemon name \"%s\"", daemonName)
		}
	}

	// ========================================================================
	// Supervisor routine - launch an independent laitos process to run daemons.
	// The command line flag is turned on by default so that laitos daemons are
	// always protected by the supervisor by default. There is no good reason
	// for a user to turn it off manually.
	// ========================================================================
	cli.HandleDaemonSignals()
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
	// The code after this point are supervised by the launcher supervisor,
	// which will automatically recover from crashes and shed components/options
	// as needed.
	if gomaxprocs > 0 {
		oldGomaxprocs := runtime.GOMAXPROCS(gomaxprocs)
		logger.Warning(nil, nil, "GOMAXPROCS has been changed from %d to %d", oldGomaxprocs, gomaxprocs)
	} else {
		logger.Warning(nil, nil, "GOMAXPROCS is unchanged at %d", runtime.GOMAXPROCS(0))
	}
	if disableConflicts {
		cli.DisableConflicts(logger)
	}
	if misc.EnableAWSIntegration {
		cli.InitialiseAWS()
	}
	cli.CopyNonEssentialUtilitiesInBackground(logger)
	cli.InstallOptionalLoggerSQSCallback(logger, config.AWSIntegration.SendWarningLogToSQSURL)

	// ========================================================================
	// Daemon routine - launch all daemons at once.
	// ========================================================================

	// Prepare environmental changes
	for _, daemonName := range daemonNames {
		// Daemons are started asynchronously and the order does not matter
		switch daemonName {
		case launcher.DNSDName:
			go cli.AutoRestart(logger, daemonName, config.GetDNSD().StartAndBlock)
		case launcher.HTTPDName:
			go cli.AutoRestart(logger, daemonName, config.GetHTTPD().StartAndBlockWithTLS)
		case launcher.InsecureHTTPDName:
			/*
				There is not an independent port settings for launching both TLS-enabled and TLS-free HTTP servers
				at the same time. If user really wishes to launch both at the same time, the TLS-free HTTP server
				will fallback to use port number 80.
			*/
			go cli.AutoRestart(logger, daemonName, func() error {
				return config.GetHTTPD().StartAndBlockNoTLS(80)
			})
		case launcher.MaintenanceName:
			go cli.AutoRestart(logger, daemonName, config.GetMaintenance().StartAndBlock)
		case launcher.PhoneHomeName:
			go cli.AutoRestart(logger, daemonName, config.GetPhoneHomeDaemon().StartAndBlock)
		case launcher.PlainSocketName:
			go cli.AutoRestart(logger, daemonName, config.GetPlainSocketDaemon().StartAndBlock)
		case launcher.SimpleIPSvcName:
			go cli.AutoRestart(logger, daemonName, config.GetSimpleIPSvcD().StartAndBlock)
		case launcher.SMTPDName:
			go cli.AutoRestart(logger, daemonName, config.GetMailDaemon().StartAndBlock)
		case launcher.SNMPDName:
			go cli.AutoRestart(logger, daemonName, config.GetSNMPD().StartAndBlock)
		case launcher.SOCKDName:
			go cli.AutoRestart(logger, daemonName, config.GetSockDaemon().StartAndBlock)
		case launcher.TelegramName:
			go cli.AutoRestart(logger, daemonName, config.GetTelegramBot().StartAndBlock)
		case launcher.AutoUnlockName:
			go cli.AutoRestart(logger, daemonName, config.GetAutoUnlock().StartAndBlock)
		case launcher.PasswdRPCName:
			go cli.AutoRestart(logger, daemonName, config.GetPasswdRPCDaemon().StartAndBlock)
		case launcher.HTTPProxyName:
			go cli.AutoRestart(logger, daemonName, config.GetHTTPProxyDaemon().StartAndBlock)
		}
	}

	// At this point the enabled daemons are running in their own background
	// goroutines. the main function now waits/blocks indefinitely.
	select {}
}
