package passwdserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/launcher"
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
	ShutdownTimeout = 10 * time.Second
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

// GetSysInfoText returns system information in human-readable text that is to be displayed on the password web page.
func GetSysInfoText() string {
	usedMem, totalMem := misc.GetSystemMemoryUsageKB()
	usedRoot, freeRoot, totalRoot := platform.GetRootDiskUsageKB()
	return fmt.Sprintf(`
Clock: %s
Sys/prog uptime: %s / %s
Total/used/prog mem: %d / %d / %d MB
Total/used/free rootfs: %d / %d / %d MB
Sys load: %s
Num CPU/GOMAXPROCS/goroutines: %d / %d / %d
`,
		time.Now().String(),
		time.Duration(misc.GetSystemUptimeSec()*int(time.Second)).String(), time.Now().Sub(misc.StartupTime).String(),
		totalMem/1024, usedMem/1024, misc.GetProgramMemoryUsageKB()/1024,
		totalRoot/1024, usedRoot/1024, freeRoot/1024,
		misc.GetSystemLoad(),
		runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.NumGoroutine())
}

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
		w.Write([]byte("OK"))
		return
	}
	switch r.Method {
	case http.MethodPost:
		ws.logger.Info("pageHandler", r.RemoteAddr, nil, "an unlock attempt has been made")

		var err error
		// Try decrypting program configuration JSON file using the input password
		key := []byte(strings.TrimSpace(r.FormValue(PasswordInputName)))
		decryptedConfig, err := misc.Decrypt(misc.ConfigFilePath, key)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, err.Error())))
			return
		}
		if decryptedConfig[0] != '{' {
			w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, "wrong key or malformed config file")))
			return
		}
		// Success!
		w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, "success")))
		ws.alreadyUnlocked = true
		// A short moment later, the function will launch laitos supervisor along with daemons.
		go ws.LaunchMainProgram([]byte(strings.TrimSpace(r.FormValue("password"))))
		return
	default:
		ws.logger.Info("pageHandler", r.RemoteAddr, nil, "just visiting")
		w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, "")))
		return
	}
}

// Start runs the web server and blocks until the server shuts down from a successful unlocking attempt.
func (ws *WebServer) Start() error {
	ws.logger = lalog.Logger{
		ComponentName: "passwdserver",
		ComponentID:   []lalog.LoggerIDField{{Key: "Port", Value: ws.Port}},
	}
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
	ws.logger.Info("Start", "", nil, "will listen on TCP port %d", ws.Port)
	if err := ws.server.ListenAndServe(); err != nil && strings.Index(err.Error(), "closed") == -1 {
		ws.logger.Warning("Start", "", err, "failed to listen on TCP port")
		return err
	}
	ws.logger.Info("Start", "", nil, "web server has stopped")
	return nil
}

// Shutdown instructs web server to shut down within several seconds, consequently that Start() function will cease to block.
func (ws *WebServer) Shutdown() error {
	shutdownTimeout, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	return ws.server.Shutdown(shutdownTimeout)
}

/*
LaunchMainProgram shuts down the web server, and forks a process of laitos program itself to launch main program using
decrypted data from ramdisk.
If an error occurs, this program will exit abnormally and the function will not return.
If the forked main program exits normally, the function will return.
*/
func (ws *WebServer) LaunchMainProgram(decryptionPassword []byte) {
	// Replicate the CLI flagsNoExec that were used to launch this password web server.
	flagsNoExec := make([]string, len(os.Args))
	copy(flagsNoExec, os.Args[1:])
	var cmd *exec.Cmd
	// Web server will take several seconds to finish with pending IO before shutting down
	if err := ws.Shutdown(); err != nil {
		ws.logger.Abort("LaunchMainProgram", "", nil, "failed to shut down web server - %v", err)
		return
	}
	// Determine path to my program
	executablePath, err := os.Executable()
	if err != nil {
		ws.logger.Abort("LaunchMainProgram", "", nil, "failed to determine path to this program executable - %v", err)
		return
	}
	// Remove CLI flags that were used to launch the web server from the flags used to launch laitos main program
	flagsNoExec = launcher.RemoveFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-"+CLIFlag)
	}, flagsNoExec)
	ws.logger.Info("LaunchMainProgram", "", nil, "about to launch with CLI flags %v", flagsNoExec)
	cmd = exec.Command(executablePath, flagsNoExec...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	launcher.FeedDecryptionPasswordToStdinAndStart(decryptionPassword, cmd)
	// Wait forever for the main program
	if err := cmd.Wait(); err != nil {
		ws.logger.Abort("LaunchMainProgram", "", nil, "main program has abnormally exited due to - %v", err)
		return
	}
	ws.logger.Info("LaunchMainProgram", "", nil, "main program has exited cleanly")
	return
}
