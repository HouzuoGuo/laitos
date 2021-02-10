package passwdserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

const (
	/*
		The constants ContentLocationMagic and PasswordInputName are copied into autounlock package in order to avoid
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

	// IOTimeout is the timeout (in seconds) used for transfering data between password input web server and clients.
	IOTimeout = 30 * time.Second
	/*
		ShutdownTimeout is the maximum number of seconds to wait for completion of pending IO transfers, before shutting
		down the password input web server.
	*/
	ShutdownTimeout = 3 * time.Second
	// CLIFlag is the command line flag that enables this password input web server to launch.
	CLIFlag = `pwdserver`
	// PageHTML is the content of HTML page that asks for a password input.
	PageHTML = `<html>
<head>
	<title>Hello</title>
</head>
<body>
	<pre>%s</pre>
    <form action="%s" method="post">
        <p>Enter password to launch main program: <input type="password" name="` + PasswordInputName + `"/></p>
        <p><input type="submit" value="Launch"/></p>
        <p>%s</p>
    </form>
</body>
</html>
	`
)

/*
WebServer runs an HTTP (not HTTPS) server that serves a single web page at a pre-designated URL, the page then allows a
visitor to enter a correct password to decrypt program data and configuration, and finally launches a supervisor along
with daemons using decrypted data.
*/
type WebServer struct {
	Port int    // Port is the TCP port to listen on.
	URL  string // URL is the secretive URL that serves the unlock page. The URL must include leading slash.

	server          *http.Server // server is the HTTP server after it is started.
	handlerMutex    *sync.Mutex  // handlerMutex prevents concurrent unlocking attempts from being made at once.
	alreadyUnlocked bool         // alreadyUnlocked is set to true after a successful unlocking attempt has been made

	logger lalog.Logger
}

/*
pageHandler serves an HTML page that allows visitor to decrypt a program data archive via a correct password.
If successful, the web server will stop, and then launches laitos supervisor program along with daemons using
configuration and data from the unencrypted (and unpacked) archive.
*/
func (ws *WebServer) pageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Location", ContentLocationMagic)
	w.Header().Set("Content-Type", "text/html")
	ws.handlerMutex.Lock()
	defer ws.handlerMutex.Unlock()
	if ws.alreadyUnlocked {
		// If an unlock attempt has already been successfully carried out, do not allow a second attempt to be made
		_, _ = w.Write([]byte("OK"))
		return
	}
	summary := platform.GetProgramStatusSummary(false)
	switch r.Method {
	case http.MethodPost:
		ws.logger.Info("pageHandler", r.RemoteAddr, nil, "an unlock attempt has been made")
		var err error
		// Try decrypting program configuration JSON file using the input password
		key := strings.TrimSpace(r.FormValue(PasswordInputName))
		decryptedConfig, err := misc.Decrypt(misc.ConfigFilePath, key)
		if err != nil {
			_, _ = w.Write([]byte(fmt.Sprintf(PageHTML, summary, r.RequestURI, err.Error())))
			return
		}
		if decryptedConfig[0] != '{' {
			_, _ = w.Write([]byte(fmt.Sprintf(PageHTML, summary, r.RequestURI, "wrong key or malformed config file")))
			return
		}
		// Success!
		_, _ = w.Write([]byte(fmt.Sprintf(PageHTML, summary, r.RequestURI, "success")))
		ws.alreadyUnlocked = true
		// The web server has no more use
		ws.Shutdown()
		misc.ProgramDataDecryptionPasswordInput <- strings.TrimSpace(r.FormValue("password"))
		return
	default:
		ws.logger.Info("pageHandler", r.RemoteAddr, nil, "just visiting")
		_, _ = w.Write([]byte(fmt.Sprintf(PageHTML, summary, r.RequestURI, "")))
		return
	}
}

// Start runs the web server and blocks until the server shuts down from a successful unlocking attempt.
func (ws *WebServer) Start() error {
	ws.logger = lalog.Logger{
		ComponentName: "passwdserver",
		ComponentID:   []lalog.LoggerIDField{{Key: "Port", Value: ws.Port}},
	}
	ws.alreadyUnlocked = false
	ws.handlerMutex = new(sync.Mutex)
	mux := http.NewServeMux()
	// Visitor must visit the pre-configured URL for a meaningful response
	mux.HandleFunc(ws.URL, ws.pageHandler)
	// All other URLs simply render an empty page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	})

	// Start web server
	ws.server = &http.Server{
		Addr:        net.JoinHostPort("0.0.0.0", strconv.Itoa(ws.Port)),
		Handler:     mux,
		ReadTimeout: IOTimeout, ReadHeaderTimeout: IOTimeout,
		WriteTimeout: IOTimeout, IdleTimeout: IOTimeout,
	}
	ws.logger.Info("Start", "", nil, "a web server has been started on port %d to collect config file decryption password at \"%s\"",
		ws.Port, ws.URL)
	if err := ws.server.ListenAndServe(); err != nil && !strings.Contains(err.Error(), "closed") {
		ws.logger.Warning("Start", "", err, "failed to listen on TCP port")
		return err
	}
	ws.logger.Info("Start", "", nil, "web server has stopped")
	return nil
}

// Shutdown instructs web server to shut down within several seconds, consequently that Start() function will cease to block.
func (ws *WebServer) Shutdown() {
	if ws.server != nil {
		shutdownTimeout, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()
		ws.logger.MaybeMinorError(ws.server.Shutdown(shutdownTimeout))
	}
	ws.server = nil
	return
}
