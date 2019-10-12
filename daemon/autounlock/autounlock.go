package autounlock

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

const (
	/*
		The constants ContentLocationMagic and PasswordInputName are copied from passwdserver package in order to avoid
		import cycle. Looks ugly, sorry.
	*/

	/*
		ContentLocationMagic is a rather randomly typed string that is sent as Content-Location header value when a
		client successfully reaches the password unlock URL (and only that URL). Clients may look for this magic
		in order to know that the URL reached indeed belongs to a laitos password input web server.
	*/
	ContentLocationMagic = "vmseuijt5oj4d5x7fygfqj4398"
	// PasswordInputName is the HTML element name that accepts password input.
	PasswordInputName = "password"
)

/*
Daemon periodically probes URLs where laitos password input servers ("passwdserver") are located in order to unlock
their program data, and submits stored passwords to those laitos URLs to unlock their data.
*/
type Daemon struct {
	URLAndPassword map[string]string `json:"URLAndPassword"` // URLAndPassword is a mapping between URL and corresponding password.
	IntervalSec    int               `json:"IntervalSec"`    // IntervalSec is the interval at which URLs are checked.

	loopIsRunning int32     // loopIsRunning has value 1 only when the daemon loop is running.
	stop          chan bool // stop signals daemon loop to stop
	logger        lalog.Logger
}

func (daemon *Daemon) Initialise() error {
	if daemon.IntervalSec < 10*60 {
		daemon.IntervalSec = 10 * 60 // 10 minutes is reasonable for almost all cases
	}
	daemon.logger = lalog.Logger{ComponentName: "autounlock", ComponentID: []lalog.LoggerIDField{{Key: "Intv", Value: daemon.IntervalSec}}}
	// Make sure that all URLs and passwords are present, and URLs can be parsed.
	for aURL, passwd := range daemon.URLAndPassword {
		if aURL == "" || passwd == "" {
			return errors.New("autounlock.Initialise: URLs and passwords must not be blank")
		}
		if _, err := url.Parse(aURL); err != nil {
			return fmt.Errorf("autounlock.Initialise: failed to parse URL \"%s\" - %v", aURL, err)
		}
	}
	daemon.stop = make(chan bool)
	return nil
}

// StartAndBlock starts the loop that probes URLs.
func (daemon *Daemon) StartAndBlock() error {
	daemon.logger.Info("StartAndBlock", "", nil, "going to probe %d URLs", len(daemon.URLAndPassword))
	for {
		if misc.EmergencyLockDown {
			atomic.StoreInt32(&daemon.loopIsRunning, 0)
			return misc.ErrEmergencyLockDown
		}
		atomic.StoreInt32(&daemon.loopIsRunning, 1)
		// Probe the URLs one after another
		for aURL, passwd := range daemon.URLAndPassword {
			parsedURL, parseErr := url.Parse(aURL)
			if parseErr == nil {
				probeResp, probeErr := inet.DoHTTP(inet.HTTPRequest{TimeoutSec: 10}, strings.Replace(aURL, "%", "%%", -1))
				if probeErr == nil && probeResp.StatusCode/200 == 1 && probeResp.Header.Get("Content-Location") == ContentLocationMagic {
					// The URL is responding successfully and is indeed a password input web server
					begin := time.Now().UnixNano()
					daemon.logger.Warning("StartAndBlock", "", nil, "trying to unlock data on domain %s", parsedURL.Host)
					// Use form submission to input password
					submitResp, submitErr := inet.DoHTTP(inet.HTTPRequest{
						// While unlocking is going on, the system is often freshly booted and quite busy, hence giving it plenty of time to respond.
						TimeoutSec:  30,
						Method:      http.MethodPost,
						ContentType: "application/x-www-form-urlencoded",
						Body:        strings.NewReader(url.Values{PasswordInputName: []string{passwd}}.Encode()),
					}, strings.Replace(aURL, "%", "%%", -1))
					if submitErr != nil {
						daemon.logger.Warning("StartAndBlock", "", submitErr, "failed to submit password to domain %s", parsedURL.Host)
					} else if submitHTTPErr := submitResp.Non2xxToError(); submitHTTPErr != nil {
						daemon.logger.Warning("StartAndBlock", "", submitHTTPErr, "failed to submit password to domain %s", parsedURL.Host)
					} else {
						daemon.logger.Warning("StartAndBlock", "", nil, "successfully unlocked domain %s, response is: %s", parsedURL.Host, submitResp.GetBodyUpTo(1024))
					}
					common.AutoUnlockStats.Trigger(float64(time.Now().UnixNano() - begin))
				}
			}
		}
		select {
		case <-daemon.stop:
			atomic.StoreInt32(&daemon.loopIsRunning, 0)
			return nil
		case <-time.After(time.Duration(daemon.IntervalSec) * time.Second):
			// Just waiting for the interval
		}
	}
}

// Stop previously started daemon loop.
func (daemon *Daemon) Stop() {
	if atomic.CompareAndSwapInt32(&daemon.loopIsRunning, 1, 0) {
		daemon.stop <- true
	}
}

func TestAutoUnlock(daemon *Daemon, t testingstub.T) {
	var unlocked bool
	// Start a web server that behaves somewhat similar to the real password input server
	pwdMatch := "this is a sample password"
	pwdURL := "/password-input"
	mux := http.NewServeMux()
	mux.HandleFunc(pwdURL, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Location", ContentLocationMagic)
		} else if r.Method == http.MethodPost {
			if r.FormValue(PasswordInputName) == pwdMatch {
				unlocked = true
				_, _ = w.Write([]byte("very good!"))
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
	// Expect HTTP server to start in a second
	time.Sleep(1 * time.Second)
	// Start the daemon and let it do the unlocking work
	/*
		Usually, the daemon configuration is made by the caller of this function, however, in this case it is not
		possible for caller to find out the port of the HTTP server above, therefore craft the configuration right here.
	*/
	daemon.URLAndPassword[fmt.Sprintf("http://localhost:%d%s", l.Addr().(*net.TCPAddr).Port, pwdURL)] = pwdMatch
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	var stopped bool
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stopped = true
	}()
	// Expect the daemon loop to unlock the server in couple of seconds
	time.Sleep(10 * time.Second)
	if !unlocked {
		t.Fatal("did not unlock")
	}
	// Expect daemon to stop in a second once it is told to stop
	daemon.Stop()
	time.Sleep(1 * time.Second)
	if !stopped {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	daemon.Stop()
	daemon.Stop()
}
