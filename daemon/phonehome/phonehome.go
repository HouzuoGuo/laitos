package phonehome

import (
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
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
)

/*
MessageProcessorServer contains password configuration and memorises execution result of the most recent
app command that the server requested this subject to run.
*/
type MessageProcessorServer struct {
	// Password is the password PIN that the server accepts for command execution.
	Password string `json:"Password"`

	// LastAppCommand is the very latest app command that message processor server asked this subject to run.
	LastAppCommand string `json:"-"`
	// LastCommandReceivedAt is the clock time of this computer upon receiving the app command from message processor server.
	LastCommandReceivedAt time.Time `json:"-"`
	// LastCommandResult is the Combined Output of app command execution result upon completion.
	LastCommandResult string `json:"-"`
	// LastCommandResult is the duration in seconds it took to run the app command.
	LastCommandDurationSec int `json:"-"`
}

/*
Daemon phones home periodically by contacting one or more store&forward message processor servers over
app command execution URLs.
*/
type Daemon struct {
	// MessageProcessorServers is a map between message processor server URL and their configuration.
	MessageProcessorServers map[string]*MessageProcessorServer `json:"MessageProcessorServers"`

	// ReportIntervalSec is the interval in seconds at which this daemon reports to the servers.
	ReportIntervalSec int `json:"ReportIntervalSec"`

	// cmdProcessor runs app commands coming in from a store&forward message processor server.
	Processor *toolbox.CommandProcessor `json:"-"`

	// stop signals StartAndBlock loop to stop processing newly connected devices.
	stop          chan bool
	loopIsRunning bool
	logger        lalog.Logger
}

// Initialise validates the daemon configuration and initalises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.ReportIntervalSec < 1 {
		daemon.ReportIntervalSec = toolbox.ReportIntervalSec
	}
	if len(daemon.MessageProcessorServers) == 0 {
		return errors.New("phonehome.Initialise: MessageProcessorServers must have at least one entry")
	}
	if daemon.Processor != nil {
		if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
			return fmt.Errorf("phonehome.Initialise: %+v", errs)
		}
	}
	daemon.stop = make(chan bool)
	daemon.logger = lalog.Logger{ComponentName: "phonehome"}
	return nil
}

// getLatestReport constructs the latest report for this system as a monitored subject.
func (daemon *Daemon) getLatestReport(server *MessageProcessorServer) (report toolbox.SubjectReportRequest, reportJSON []byte) {
	hostname, _ := os.Hostname()
	report = toolbox.SubjectReportRequest{
		SubjectIP:       inet.GetPublicIP(),
		SubjectHostName: hostname,
		SubjectPlatform: fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH),
		SubjectComment:  toolbox.GetRuntimeInfo(),
		CommandResponse: toolbox.AppCommandResponse{
			Command:        server.LastAppCommand,
			ReceivedAt:     server.LastCommandReceivedAt,
			Result:         server.LastCommandResult,
			RunDurationSec: server.LastCommandDurationSec,
		},
	}
	// Serialise the report into JSON as the app command input
	var err error
	reportJSON, err = json.Marshal(report)
	if err != nil {
		daemon.logger.Warning("getLatestReport", "", err, "failed to generate the latest report")
	}
	return
}
func (daemon *Daemon) getTwoFACode(server *MessageProcessorServer) string {
	// The first 2FA is calculated from the command password
	_, cmdPassword1, _, err := toolbox.GetTwoFACodes(server.Password)
	if err != nil {
		daemon.logger.Warning("getTwoFACode", "", err, "failed to generate the first 2FA")
		return ""
	}
	// The second 2FA is calculated from the reversed command password
	reversedPass := []rune(server.Password)
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

// StartAndBlock starts the periodic reports and blocks caller until the daemon is stopped.
func (daemon *Daemon) StartAndBlock() error {
	defer func() {
		daemon.loopIsRunning = false
	}()
	daemon.loopIsRunning = true
	daemon.logger.Info("StartAndBlock", "", nil, "reporting to %d URLs", len(daemon.MessageProcessorServers))
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
		allURLs := make([]string, 0, len(daemon.MessageProcessorServers))
		for cmdURL := range daemon.MessageProcessorServers {
			allURLs = append(allURLs, cmdURL)
		}
		rand.Shuffle(len(allURLs), func(i, j int) { allURLs[i], allURLs[j] = allURLs[j], allURLs[i] })

		for _, cmdURL := range allURLs {
			srv := daemon.MessageProcessorServers[cmdURL]
			_, reportJSON := daemon.getLatestReport(srv)
			// Send the HTTP request
			resp, err := inet.DoHTTP(inet.HTTPRequest{
				TimeoutSec: 15,
				MaxBytes:   16 * 1024,
				Method:     http.MethodPost,
				Body: strings.NewReader(url.Values{
					"cmd": {daemon.getTwoFACode(srv) + toolbox.StoreAndForwardMessageProcessorTrigger + string(reportJSON)},
				}.Encode()),
			}, cmdURL)
			if err != nil {
				daemon.logger.Warning("StartAndBlock", cmdURL, err, "failed to send HTTP request")
				continue
			}
			var reportResponse toolbox.SubjectReportResponse
			if err := json.Unmarshal(resp.Body, &reportResponse); err != nil {
				daemon.logger.Warning("StartAndBlock", cmdURL, err, "failed to deserialise JSON report response - %s", resp.GetBodyUpTo(200))
				continue
			}
			if newCmd := reportResponse.CommandRequest.Command; newCmd != "" && daemon.Processor != nil {
				if newCmd != srv.LastAppCommand || srv.LastCommandReceivedAt.Before(time.Now().Add(-toolbox.CommandResponseRetentionSec*time.Second)) {
					// Execute the command sent back by the server
					daemon.logger.Info("StartAndBlock", cmdURL, nil, "running app command from report reply")
					srv.LastAppCommand = newCmd
					srv.LastCommandReceivedAt = time.Now()
					// By convention duration of -1 indicates that the command is yet to complete
					srv.LastCommandDurationSec = -1
					// Run the app command in background to avoid blocking this loop
					go func(srv *MessageProcessorServer, newCmd string) {
						start := time.Now()
						result := daemon.Processor.Process(toolbox.Command{
							// The "client" that asked for this app command is the message processor server
							ClientID:   cmdURL,
							DaemonName: "phonehome",
							TimeoutSec: toolbox.CommandResponseRetentionSec,
							Content:    newCmd,
						}, true)
						srv.LastAppCommand = newCmd
						srv.LastCommandDurationSec = int(time.Now().Unix() - start.Unix())
						srv.LastCommandResult = result.CombinedOutput
					}(srv, newCmd)
				} else {
					daemon.logger.Info("StartAndBlock", cmdURL, nil,
						"skipping the app command from report reply as it was run quite recently at %s", srv.LastCommandReceivedAt)
				}
			} else {
				daemon.logger.Info("StartAndBlock", cmdURL, nil, "report sent")
			}
		}
		/*
			Sleep for the interval and continue to the next round.
			The actual reporting interval is not going to be exactly the recommended interval due to latency with IO.
		*/
		select {
		case <-daemon.stop:
			return nil
		case <-time.After(time.Duration(daemon.ReportIntervalSec) * time.Second):
		}
	}
}

// Stop the daemon.
func (daemon *Daemon) Stop() {
	if daemon.loopIsRunning {
		daemon.loopIsRunning = false
		daemon.stop <- true
	}
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
				result := muxMessageProcessor.Execute(toolbox.Command{
					Content:    reqCmd[strings.Index(reqCmd, ".0m")+3:],
					TimeoutSec: 2,
					ClientID:   r.RemoteAddr,
					DaemonName: "httpd",
				})
				t.Log(reqCmd)
				if result.Error != nil {
					t.Fatalf("1st request error: %+v", result)
				}
				if len(muxMessageProcessor.OutstandingCommands) > 0 {
					t.Fatalf("1st request unexpected outstanding command: %+v", muxMessageProcessor.OutstandingCommands)
				}
				if len(muxMessageProcessor.SubjectReports) != 1 {
					t.Fatalf("1st request unexpected subject reports: %+v", muxMessageProcessor.SubjectReports)
				}
				for _, reports := range muxMessageProcessor.SubjectReports {
					report0 := (*reports)[0]
					if report0.SubjectClientIP == "" || report0.DaemonName == "" || report0.OriginalRequest.SubjectHostName == "" ||
						report0.OriginalRequest.CommandRequest.Command != "" || report0.OriginalRequest.CommandResponse.Command != "" {
						t.Fatalf("1st request, unexpected memorised report: %+v", report0)
					}
				}
				// The response will ask the daemon to run an app command
				resp := toolbox.SubjectReportResponse{
					CommandRequest: toolbox.AppCommandRequest{
						Command: toolbox.TestCommandProcessorPIN + ".s echo hi",
					},
				}
				respJSON, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = w.Write(respJSON)
				muxNumRequests++
			} else if muxNumRequests == 1 {
				// The second request is a report that carries the app execution result from the app command.
				reqCmd := r.FormValue("cmd")
				result := muxMessageProcessor.Execute(toolbox.Command{
					Content:    reqCmd[strings.Index(reqCmd, ".0m")+3:],
					TimeoutSec: 2,
					ClientID:   r.RemoteAddr,
					DaemonName: "httpd",
				})
				t.Log(reqCmd)
				if result.Error != nil {
					t.Fatalf("2st request error: %+v", result)
				}
				if len(muxMessageProcessor.OutstandingCommands) > 0 {
					t.Fatalf("2st request unexpected outstanding command: %+v", muxMessageProcessor.OutstandingCommands)
				}
				if len(muxMessageProcessor.SubjectReports) != 1 {
					t.Fatalf("2st request unexpected subject reports: %+v", muxMessageProcessor.SubjectReports)
				}
				for _, reports := range muxMessageProcessor.SubjectReports {
					report1 := (*reports)[1]
					if report1.SubjectClientIP == "" || report1.DaemonName == "" || report1.OriginalRequest.SubjectHostName == "" ||
						report1.OriginalRequest.CommandRequest.Command != "" || report1.OriginalRequest.CommandResponse.Command == "" ||
						report1.OriginalRequest.CommandResponse.Result != "hi" || report1.OriginalRequest.CommandResponse.RunDurationSec > 2 {
						t.Fatalf("2nd request, unexpected memorised report: %+v", report1)
					}
				}
				// Validate the command execution from daemon's perspective
				for _, mp := range server.MessageProcessorServers {
					if mp.LastAppCommand == "" || mp.LastCommandDurationSec < 0 || mp.LastCommandResult != "hi" {
						t.Fatalf("2nd request, unexpected result from daemon: %+v", mp)
					}
				}

				// The response will ask the daemon to run an app command
				resp := toolbox.SubjectReportResponse{
					CommandRequest: toolbox.AppCommandRequest{
						Command: toolbox.TestCommandProcessorPIN + ".s echo hi",
					},
				}
				respJSON, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = w.Write(respJSON)
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
	server.MessageProcessorServers = map[string]*MessageProcessorServer{
		fmt.Sprintf("http://localhost:%d/test", l.Addr().(*net.TCPAddr).Port): &MessageProcessorServer{
			Password: toolbox.TestCommandProcessorPIN,
		},
	}
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
	// Daemon should stop within a second
	server.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	server.Stop()
	server.Stop()
}
