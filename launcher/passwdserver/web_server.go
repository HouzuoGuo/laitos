package passwdserver

import (
	"context"
	"fmt"
	"github.com/HouzuoGuo/laitos/launcher/encarchive"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// IOTimeout is the timeout (in seconds) used in password launcher's web server transfers.
	IOTimeout = 30 * time.Second
	/*
		ShutdownTimeout is the maximum number of seconds to wait for completion of pending IO transfers,
		before shutting down the password launcher's web server.
	*/
	ShutdownTimeout = 10 * time.Second
	CLIArgument     = `sl` // CLIArgument as a CLI parameter triggers this special launching mechanism.

	PageHTML = `<!doctype html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
	<title>Hello</title>
</head>
<body>
	<pre>%s</pre>
    <form action="#" method="post">
        <p>Enter password to launch main program: <input type="password" name="password"/></p>
        <p><input type="submit" value="Launch"/></p>
        <p>%s</p>
    </form>
</body>
</html>
	` // PageHTML is the content of HTML page that asks for a password to decrypt and launch main program.
)

// GetSysInfoText returns system information in human-readable text that is to be displayed on the password web page.
func GetSysInfoText() string {
	usedMem, totalMem := misc.GetSystemMemoryUsageKB()
	return fmt.Sprintf(`
Clock: %s
Sys/prog uptime: %s / %s
Total/used/prog mem: %d / %d / %d MB
Sys load: %s
Num CPU/GOMAXPROCS/goroutines: %d / %d / %d
`,
		time.Now().String(),
		time.Duration(misc.GetSystemUptimeSec()*int(time.Second)).String(), time.Now().Sub(misc.StartupTime).String(),
		totalMem/1024, usedMem/1024, misc.GetProgramMemoryUsageKB()/1024,
		misc.GetSystemLoad(),
		runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.NumGoroutine())
}

/*
WebServer runs an HTTP (not HTTPS) server that serves a single web page at a pre-designated URL, the page then allows a
visitor to enter a correct password to decrypt program data and configuration, and finally launches a supervisor along
with daemons using decrypted data.
*/
type WebServer struct {
	Port            int    // Port is the TCP port to listen on.
	URL             string // URL is the secretive URL that serves the unlock page. The URL must include leading slash.
	ArchiveFilePath string // ArchiveFilePath is the absolute or relative path to encrypted archive file.

	server          *http.Server // server is the HTTP server after it is started.
	archiveFileSize int          // archiveFileSize is the size of the archive file, it is set when web server starts.
	ramdiskDir      string       // ramdiskDir is set after archive has been successfully extracted.
	handlerMutex    *sync.Mutex  // handlerMutex prevents concurrent operations on ramdisk.
	logger          *misc.Logger
}

/*
pageHandler serves an HTML page that allows visitor to decrypt an archive via a correct password.
If successful, the web server will stop, and then launches laitos supervisor program along with daemons using
configuration and data from the unencrypted (and unpacked) archive.
*/
func (ws *WebServer) pageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	switch r.Method {
	case http.MethodPost:
		ws.logger.Printf("pageHandler", r.RemoteAddr, nil, "an unlock attempt has been made")
		ws.handlerMutex.Lock()
		// Ramdisk size in MB = archive size (unencrypted archive) + archive size (extracted files) + 8 (just in case)
		var err error
		ws.ramdiskDir, err = encarchive.MakeRamdisk(ws.archiveFileSize/1048576*2 + 8)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), err.Error())))
			ws.handlerMutex.Unlock()
			return
		}
		// Create extract temp file inside ramdisk
		tmpFile, err := ioutil.TempFile(ws.ramdiskDir, "launcher-extract-temp-file")
		if err != nil {
			w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), err.Error())))
			ws.handlerMutex.Unlock()
			return
		}
		defer tmpFile.Close()
		defer os.Remove(tmpFile.Name())
		// Extract files into ramdisk
		if err := encarchive.Extract(ws.ArchiveFilePath, tmpFile.Name(), ws.ramdiskDir, []byte(strings.TrimSpace(r.FormValue("password")))); err != nil {
			encarchive.DestroyRamdisk(ws.ramdiskDir)
			w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), err.Error())))
			ws.handlerMutex.Unlock()
			return
		}
		// Success! Do not unlock handlerMutex anymore because there is no point in visiting this handler again.
		w.Write([]byte(fmt.Sprintf(PageHTML, "success")))
		// A short moment later, the function will launch laitos supervisor along with daemons.
		go ws.LaunchSupervisorUsingDecryptedData()
		return
	default:
		ws.logger.Printf("pageHandler", r.RemoteAddr, nil, "just visiting")
		w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), "")))
		return
	}
}

// Start runs the web server and blocks until the server shuts down from a successful unlocking attempt.
func (ws *WebServer) Start() error {
	ws.logger = &misc.Logger{
		ComponentName: "passwdserver.WebServer",
		ComponentID:   strconv.Itoa(ws.Port),
	}
	ws.handlerMutex = new(sync.Mutex)
	// Page handler needs to know the size in order to prepare ramdisk
	stat, err := os.Stat(ws.ArchiveFilePath)
	if err != nil {
		ws.logger.Warningf("Start", "", err, "failed to read archive file at %s", ws.ArchiveFilePath)
		return err
	}
	ws.archiveFileSize = int(stat.Size())

	mux := http.NewServeMux()
	// Visitor must visit the pre-configured URL for a meaningful response
	mux.HandleFunc(ws.URL, ws.pageHandler)
	// All other URLs simply render an empty page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	})
	ws.server = &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%d", ws.Port), Handler: mux,
		ReadTimeout: IOTimeout, ReadHeaderTimeout: IOTimeout,
		WriteTimeout: IOTimeout, IdleTimeout: IOTimeout,
	}
	ws.logger.Printf("Start", "", nil, "will listen on TCP port %d", ws.Port)
	if err := ws.server.ListenAndServe(); err != nil && strings.Index(err.Error(), "closed") == -1 {
		ws.logger.Warningf("Start", "", err, "failed to listen on TCP port")
		return err
	}
	ws.logger.Printf("Start", "", nil, "web server has stopped")
	return nil
}

// Shutdown instructs web server to shut down within several seconds, consequently that Start() function will cease to block.
func (ws *WebServer) Shutdown() error {
	shutdownTimeout, _ := context.WithTimeout(context.Background(), ShutdownTimeout)
	return ws.server.Shutdown(shutdownTimeout)
}

/*
LaunchSupervisorUsingDecryptedData shuts down the web server, and forks a process of laitos program itself to launch
supervisor along with daemons using decrypted data from ramdisk. If errors occure, the program will exit abnormally.
*/
func (ws *WebServer) LaunchSupervisorUsingDecryptedData() {
	var fatalMsg string
	execPath, err := os.Executable()
	args := make([]string, 0, 8)
	var cmd *exec.Cmd
	// Web server will take several seconds to finish with pending IO before shutting down
	if err = ws.Shutdown(); err != nil {
		fatalMsg = fmt.Sprintf("failed to determine executable path - %v", err)
		goto fatalExit
	}
	if err != nil {
		fatalMsg = fmt.Sprintf("failed to determine executable path - %v", err)
		goto fatalExit
	}
	// Switch to the ramdisk directory full of decrypted data for launching supervisor and daemons
	if err := os.Chdir(ws.ramdiskDir); err != nil {
		fatalMsg = fmt.Sprintf("failed to cd to %s - %v", ws.ramdiskDir, err)
		goto fatalExit
	}
	// Replicate the CLI arguments that were used to launch this launcher
	for i, arg := range os.Args {
		if i != 0 && arg != "-"+CLIArgument {
			args = append(args, arg)
		}
	}
	ws.logger.Printf("LaunchSupervisorUsingDecryptedData", "", nil, "about to relaunch myself with args %v", args)
	cmd = exec.Command(execPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatalMsg = fmt.Sprintf("failed to launch self - %v", err)
		goto fatalExit
	}
	if err := cmd.Wait(); err != nil {
		fatalMsg = fmt.Sprintf("supervisor has abnormally exited due to - %v", err)
		goto fatalExit
	}
	ws.logger.Printf("LaunchSupervisorUsingDecryptedData", "", nil, "supervisor has exited cleanly")
	// In both normal and abnormal paths, the ramdisk must be destroyed.
	encarchive.DestroyRamdisk(ws.ramdiskDir)
	return
fatalExit:
	encarchive.DestroyRamdisk(ws.ramdiskDir)
	ws.logger.Fatalf("LaunchSupervisorUsingDecryptedData", "", nil, fatalMsg)
}
