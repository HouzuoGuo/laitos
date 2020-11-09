package phonehome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
)

/*
MessageProcessorServer contains server and password password configuration. If the server has an HTTP Endpoint URL,
the report will be sent via an HTTP client. Otherwise if the server has a DNS domain name, the report will be sent
via DNS TXT query.
*/
type MessageProcessorServer struct {
	/*
		HTTPEndpointURL is the complete URL of endpoint HandleAppCommand that will receive subject reports.
		If this is set, then DNSDomainName will be ignored.
	*/
	HTTPEndpointURL string `json:"HTTPEndpointURL"`
	/*
		DNSDomainName is the domain name where laitos DNS server runs to receive subject reports.
		If this is set, then HTTPEndpointURL will be ignored.
	*/
	DNSDomainName string `json:"DNSDomainName"`
	// Password is the password PIN that the server accepts for command execution.
	Passwords []string `json:"Passwords"`
	// HostName is the host name portion of server app command execution URL, it is calculated by Initialise function.
	HostName string `json:"-"`
}

/*
Daemon phones home periodically by contacting one or more store&forward message processor servers over
app command execution URLs.
*/
type Daemon struct {
	// MessageProcessorServers is a map between message processor server URL and their configuration.
	MessageProcessorServers []*MessageProcessorServer `json:"MessageProcessorServers"`

	// ReportIntervalSec is the interval in seconds at which this daemon reports to the servers.
	ReportIntervalSec int `json:"ReportIntervalSec"`

	// LocalMessageProcessor answers to servers' app command requests
	LocalMessageProcessor *toolbox.MessageProcessor `json:"-"`
	// cmdProcessor runs app commands coming in from a store&forward message processor server.
	Processor *toolbox.CommandProcessor `json:"-"`

	runLoop bool
	logger  lalog.Logger
}

// Initialise validates the daemon configuration and initalises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.ReportIntervalSec < 1 {
		daemon.ReportIntervalSec = toolbox.ReportIntervalSec
	}
	if len(daemon.MessageProcessorServers) == 0 {
		return errors.New("phonehome.Initialise: MessageProcessorServers must have at least one entry")
	}
	/*
		Daemon's app command processor is not used directly, instead it is given to the local message processor
		to process app commands requested by remote server.
	*/
	if daemon.Processor != nil {
		if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
			return fmt.Errorf("phonehome.Initialise: %+v", errs)
		}
	}
	// There is no point in keeping many app command exchange reports in memory
	daemon.LocalMessageProcessor = &toolbox.MessageProcessor{
		OwnerName:             "phonehome",
		MaxReportsPerHostName: 10,
		CmdProcessor:          daemon.Processor,
	}
	if err := daemon.LocalMessageProcessor.Initialise(); err != nil {
		return fmt.Errorf("phonehome.Initialise: failed to initialise local message processor - %v", err)
	}
	for _, srv := range daemon.MessageProcessorServers {
		if srv.DNSDomainName == "" && srv.HTTPEndpointURL == "" {
			return fmt.Errorf("phonehome.Initialise: a server configuration is missing both DNSDomainName and HTTPEndpointURL")
		}
		if len(srv.Passwords) == 0 {
			return fmt.Errorf("phonehome.Initialise: server configuration for %s must contain one or more app command execution password", srv.DNSDomainName+srv.HTTPEndpointURL)
		}
		srv.HostName = srv.DNSDomainName
		if srv.HTTPEndpointURL != "" {
			// Calculate the host name portion of each URL, the host name is used by the local message processor.
			u, err := url.Parse(srv.HTTPEndpointURL)
			if err != nil {
				return fmt.Errorf("phonehome.Initialise: malformed app command URL \"%s\"", srv.HTTPEndpointURL)
			}
			srv.HostName = u.Hostname()
		}
	}
	daemon.logger = lalog.Logger{ComponentName: "phonehome"}
	return nil
}

func (daemon *Daemon) getTwoFACode(server *MessageProcessorServer) string {
	// The first 2FA is calculated from the command password
	accessPassword := server.Passwords[rand.Intn(len(server.Passwords))]
	_, cmdPassword1, _, err := toolbox.GetTwoFACodes(accessPassword)
	if err != nil {
		daemon.logger.Warning("getTwoFACode", "", err, "failed to generate the first 2FA")
		return ""
	}
	// The second 2FA is calculated from the reversed command password
	reversedPass := []rune(accessPassword)
	for i, j := 0, len(reversedPass)-1; i < j; i, j = i+1, j-1 {
		reversedPass[i], reversedPass[j] = reversedPass[j], reversedPass[i]
	}
	_, cmdPassword2, _, err := toolbox.GetTwoFACodes(string(reversedPass))
	if err != nil {
		daemon.logger.Warning("getTwoFACode", "", err, "failed to generate the second 2FA")
		return ""
	}
	return cmdPassword1 + cmdPassword2
}

func (daemon *Daemon) getReportForServer(serverHostName string, shortenMyHostName bool) string {
	// Ask local message processor for a pending app command request and/or app command response
	cmdExchange := daemon.LocalMessageProcessor.StoreReport(context.Background(), toolbox.SubjectReportRequest{SubjectHostName: serverHostName}, serverHostName, "phonehome")
	// Craft the report for this server
	hostname, _ := os.Hostname()
	if shortenMyHostName && len(hostname) > 16 {
		// Shorten the host name for a report transmitted via DNS. Length of 16 looks familiar to the nostalgic NetBIOS users.
		hostname = hostname[:16]
	}
	report := toolbox.SubjectReportRequest{
		SubjectIP:       inet.GetPublicIP(),
		SubjectHostName: strings.ToLower(hostname),
		SubjectPlatform: fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH),
		SubjectComment:  platform.GetSysSummary(true),
		CommandRequest:  cmdExchange.CommandRequest,
		CommandResponse: cmdExchange.CommandResponse,
	}
	return report.SerialiseCompact()
}

// StartAndBlock starts the periodic reports and blocks caller until the daemon is stopped.
func (daemon *Daemon) StartAndBlock() error {
	defer func() {
		daemon.runLoop = false
	}()
	/*
		Instead of sending numerous reports in a row and then wait for a longer duration, send one report at a time and
		wait a shorter duration. This helps to reduce server load and overall offers more reliability.
		If there is a large number of servers to contact, the minimum interval will be one second.
	*/
	intervalSecBetweenReports := daemon.ReportIntervalSec / len(daemon.MessageProcessorServers)
	if intervalSecBetweenReports < 1 {
		intervalSecBetweenReports = 1
	}
	daemon.logger.Info("StartAndBlock", "", nil, "reporting to %d servers and pausing %d seconds between each",
		len(daemon.MessageProcessorServers), intervalSecBetweenReports)
	daemon.runLoop = true
	for {
		if misc.EmergencyLockDown {
			daemon.logger.Warning("StartAndBlock", "", misc.ErrEmergencyLockDown, "")
			return misc.ErrEmergencyLockDown
		}
		/*
			Shuffle the destination URLs that reports are sent to.
			Reports are sent using 2FA authentication rather than the regular password authentication, if destinations
			are not contacted in a random order, there is a chance that the daemon may reach its own server first (this
			is a valid configuration) and it will always reject further reports as 2FA codes cannot be used a second time.
		*/
		srvIndexes := make([]int, 0, len(daemon.MessageProcessorServers))
		for i := range daemon.MessageProcessorServers {
			srvIndexes = append(srvIndexes, i)
		}
		rand.Shuffle(len(srvIndexes), func(i, j int) { srvIndexes[i], srvIndexes[j] = srvIndexes[j], srvIndexes[i] })

		for _, i := range srvIndexes {
			if !daemon.runLoop {
				return nil
			}
			time.Sleep(time.Duration(intervalSecBetweenReports) * time.Second)
			srv := daemon.MessageProcessorServers[i]
			var reportResponseJSON []byte
			if srv.DNSDomainName != "" {
				// Send the latest report via DNS name query
				reportCmd := daemon.getTwoFACode(srv) + toolbox.StoreAndForwardMessageProcessorTrigger + daemon.getReportForServer(srv.HostName, true)
				queryResponse, err := net.LookupTXT(GetDNSQuery(reportCmd, srv.DNSDomainName))
				if err != nil {
					daemon.logger.Warning("StartAndBlock", srv.DNSDomainName, err, "failed to send DNS request")
					continue
				}
				reportResponseJSON = []byte(strings.Join(queryResponse, ""))
			} else if srv.HTTPEndpointURL != "" {
				// Send the latest report via HTTP client
				reportCmd := daemon.getTwoFACode(srv) + toolbox.StoreAndForwardMessageProcessorTrigger + daemon.getReportForServer(srv.HostName, false)
				resp, err := inet.DoHTTP(context.Background(), inet.HTTPRequest{
					TimeoutSec: 15,
					MaxBytes:   16 * 1024,
					Method:     http.MethodPost,
					Body:       strings.NewReader(url.Values{"cmd": {reportCmd}}.Encode()),
				}, srv.HTTPEndpointURL)
				if err != nil {
					daemon.logger.Warning("StartAndBlock", srv.HTTPEndpointURL, err, "failed to send HTTP request")
					continue
				}
				reportResponseJSON = resp.Body
			}
			// Deserialise the server JSON response and pass it to local message processor to process the command request
			var reportResponse toolbox.SubjectReportResponse
			if err := json.Unmarshal(reportResponseJSON, &reportResponse); err != nil {
				daemon.logger.Info("StartAndBlock", srv.DNSDomainName+srv.HTTPEndpointURL, nil, "failed to deserialise JSON report response - %s", string(reportResponseJSON))
				continue
			}
			daemon.LocalMessageProcessor.StoreReport(context.TODO(), toolbox.SubjectReportRequest{
				SubjectHostName: srv.HostName,
				ServerTime:      time.Time{},
				CommandRequest:  reportResponse.CommandRequest,
				CommandResponse: reportResponse.CommandResponse,
			}, srv.HostName, "phonehome")
			daemon.logger.Info("StartAndBlock", srv.HostName, nil, "report sent")
		}
	}
}

// Stop the daemon.
func (daemon *Daemon) Stop() {
	daemon.runLoop = false
}

// TestServer implements test cases for the phone-home daemon.
func TestServer(server *Daemon, t testingstub.T) {
	// Start a web server that behaves like a message processor server
	mux := http.NewServeMux()
	muxNumRequests := 0
	muxMessageProcessor := toolbox.MessageProcessor{CmdProcessor: toolbox.GetTestCommandProcessor()}
	if err := muxMessageProcessor.Initialise(); err != nil {
		t.Fatal(err)
	}
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		// The handler is designed to handle exactly two requests
		if r.Method == http.MethodPost {
			if muxNumRequests == 0 {
				// The first request is expected to be a regular report
				reqCmd := r.FormValue("cmd")
				result := muxMessageProcessor.Execute(context.Background(), toolbox.Command{
					Content:    reqCmd[strings.Index(reqCmd, ".0m")+3:],
					TimeoutSec: 2,
					ClientID:   r.RemoteAddr,
					DaemonName: "httpd",
				})
				t.Log("1st req:", reqCmd)
				if result.Error != nil {
					t.Fatalf("1st request error: %+v", result)
				}
				if len(muxMessageProcessor.IncomingAppCommands) != 1 { // ".s echo 2server"
					t.Fatalf("1st request unexpected incoming command: %+v", muxMessageProcessor.IncomingAppCommands)
				}
				if len(muxMessageProcessor.SubjectReports) != 1 {
					t.Fatalf("1st request unexpected subject reports: %+v", muxMessageProcessor.SubjectReports)
				}
				for _, reports := range muxMessageProcessor.SubjectReports {
					// Verify the collected report details
					report0 := (*reports)[0]
					if report0.SubjectClientID == "" || report0.DaemonName == "" || report0.OriginalRequest.SubjectHostName == "" ||
						!strings.Contains(report0.OriginalRequest.SubjectComment, "IP:") || !strings.Contains(report0.OriginalRequest.SubjectComment, "Program flags:") ||
						report0.OriginalRequest.CommandRequest.Command != toolbox.TestCommandProcessorPIN+".s echo 2server" {
						t.Fatalf("1st request, unexpected memorised report: %+v", report0)
					}
				}
				// The response will ask the daemon to run an app command
				resp := toolbox.SubjectReportResponse{
					CommandRequest: toolbox.AppCommandRequest{
						Command: toolbox.TestCommandProcessorPIN + ".s echo 2client",
					},
				}
				respJSON, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}
				t.Log("1st resp:", string(respJSON))
				_, _ = w.Write(respJSON)
				muxNumRequests++
			} else if muxNumRequests == 1 {
				// The second request is a report that carries the app execution result from the app command.
				reqCmd := r.FormValue("cmd")
				result := muxMessageProcessor.Execute(context.Background(), toolbox.Command{
					Content:    reqCmd[strings.Index(reqCmd, ".0m")+3:],
					TimeoutSec: 2,
					ClientID:   r.RemoteAddr,
					DaemonName: "httpd",
				})
				t.Log("2nd req:", reqCmd)
				if result.Error != nil {
					t.Fatalf("2st request error: %+v", result)
				}
				if len(muxMessageProcessor.IncomingAppCommands) != 1 { // "local-to-server"
					t.Fatalf("2st request unexpected incoming command: %+v", muxMessageProcessor.IncomingAppCommands)
				}
				if len(muxMessageProcessor.SubjectReports) != 1 {
					t.Fatalf("2st request unexpected subject reports: %+v", muxMessageProcessor.SubjectReports)
				}
				for _, reports := range muxMessageProcessor.SubjectReports {
					report1 := (*reports)[1]
					if report1.SubjectClientID == "" || report1.DaemonName == "" || report1.OriginalRequest.SubjectHostName == "" ||
						!strings.Contains(report1.OriginalRequest.SubjectComment, "Clock:") || !strings.Contains(report1.OriginalRequest.SubjectComment, "Program flags:") ||
						report1.OriginalRequest.CommandRequest.Command != toolbox.TestCommandProcessorPIN+".s echo 2server" ||
						report1.OriginalRequest.CommandResponse.Command != toolbox.TestCommandProcessorPIN+".s echo 2client" ||
						report1.OriginalRequest.CommandResponse.Result != "2client" || report1.OriginalRequest.CommandResponse.RunDurationSec < 0 {
						t.Fatalf("2nd request, unexpected memorised report: %+v", report1)
					}
				}

				// The response will ask the daemon to run an app command
				_, _ = w.Write([]byte(result.Output))
				t.Log("2nd resp:", result.Output)
				muxNumRequests++
			}
		}
	})
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := http.Server{Addr: "0.0.0.0:0", Handler: mux}
	go func() {
		if err := srv.Serve(l); err != nil {
			t.Fatal(err)
		}
	}()
	// Start phone-home daemon
	cmdURL := fmt.Sprintf("http://localhost:%d/test", l.Addr().(*net.TCPAddr).Port)
	server.MessageProcessorServers = []*MessageProcessorServer{
		{Passwords: []string{toolbox.TestCommandProcessorPIN}, HTTPEndpointURL: cmdURL},
		{Passwords: []string{toolbox.TestCommandProcessorPIN}, DNSDomainName: "example.com"},
	}
	if err := server.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Prepare an outgoing to be sent to the server by the local message processor
	server.LocalMessageProcessor.SetOutgoingCommand("localhost", toolbox.TestCommandProcessorPIN+".s echo 2server")
	var stoppedNormally bool
	go func() {
		if err := server.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	// The daemon is expected to run at 1 second interval and the web server tests the request/response sequences
	time.Sleep(5 * time.Second)
	if muxNumRequests < 2 {
		t.Fatalf("did not hit test server - got %d requests", muxNumRequests)
	}
	// Check local message processor's number of reports
	localReports := server.LocalMessageProcessor.GetLatestReports(1000)
	if len(localReports) < 5 {
		t.Fatalf("%d\n%+v", len(localReports), localReports)
	}
	// Check server response from the 2nd request
	lastReport := localReports[0]
	t.Logf("phonehome latest report: %+v", lastReport)
	if lastReport.SubjectClientID != "localhost" || time.Now().Unix()-lastReport.ServerTime.Unix() > 10 || lastReport.DaemonName != "phonehome" ||
		lastReport.OriginalRequest.CommandRequest.Command != "" ||
		lastReport.OriginalRequest.CommandResponse.Command != toolbox.TestCommandProcessorPIN+".s echo 2server" ||
		lastReport.OriginalRequest.CommandResponse.RunDurationSec < 0 ||
		lastReport.OriginalRequest.CommandResponse.Result != "2server" {
		t.Fatalf("%+v", lastReport)
	}
	// Daemon should stop shortly
	server.Stop()
	time.Sleep(2 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	server.Stop()
	server.Stop()
}
