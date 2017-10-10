package encarchive

import (
	"context"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	HTTPTimeout = 30 * time.Second // HTTPTimeout is the timeout used in HTTP request/response/shutdown operations.
	MagicArg    = `sl`             // LauncherMagicArg as a CLI parameter triggers this special launching mechanism.

	UnlockPageHTML = `<!doctype html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
	<title>Hello</title>
</head>
<body>
    <form action="#" method="post">
        <p><input type="password" name="password"/></p>
        <p><input type="submit" value="Launch"/></p>
        <p>%s</p>
    </form>
</body>
</html>
	` // UnlockPageHTML is the HTML page of the archive unlocking page.
)

/*
WebServer runs an HTTP (not HTTPS) server that serves a page to allow visitor to unlock archive and then launch laitos.
*/
type WebServer struct {
	Port            int    // Port is the TCP port to listen on.
	URL             string // URL is the secretive URL that serves the unlock page. The URL must include leading slash.
	ArchiveFilePath string // ArchiveFilePath is the absolute or relative path to encrypted archive file.

	server          *http.Server // server is the HTTP server after it is started.
	archiveFileSize int          // archiveFileSize is the size of the archive file, it is set when web server starts.
	ramdiskDir      string       // ramdiskDir is set after archive has been successfully extracted.
	handlerMutex    *sync.Mutex  // handlerMutex prevents concurrent operations on ramdisk.
	logger          *misc.Logger // logger
}

/*
pageHandler serves an HTML page that allows visitor to unlock an archive via a correct password.
If successful , the server quits and launches laitos program using configuration and data from the unencrypted and
unpacked archive.
*/
func (ws *WebServer) pageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	switch r.Method {
	case http.MethodPost:
		ws.logger.Printf("pageHandler", r.RemoteAddr, nil, "an unlock attempt has been made")
		ws.handlerMutex.Lock()
		// Ramdisk size in MB = archive size (tmp output) + archive size (extracted) + 8 (just in case)
		var err error
		ws.ramdiskDir, err = MakeRamdisk(ws.archiveFileSize/1048576*2 + 8)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(UnlockPageHTML, err.Error())))
			ws.handlerMutex.Unlock()
			return
		}
		// Create extract temp file inside ramdisk
		tmpFile, err := ioutil.TempFile(ws.ramdiskDir, "launcher-extract-temp-file")
		if err != nil {
			w.Write([]byte(fmt.Sprintf(UnlockPageHTML, err.Error())))
			ws.handlerMutex.Unlock()
			return
		}
		defer tmpFile.Close()
		defer os.Remove(tmpFile.Name())
		// Extract files into ramdisk
		if err := Extract(ws.ArchiveFilePath, tmpFile.Name(), ws.ramdiskDir, []byte(strings.TrimSpace(r.FormValue("password")))); err != nil {
			DestroyRamdisk(ws.ramdiskDir)
			w.Write([]byte(fmt.Sprintf(UnlockPageHTML, err.Error())))
			ws.handlerMutex.Unlock()
			return
		}
		// Success! Do not unlock handlerMutex anymore because there is no point in visiting this handler again.
		w.Write([]byte(fmt.Sprintf(UnlockPageHTML, "success")))
		// A short moment later, launch laitos program using the unlocked archive.
		go ws.LaunchWithUnlockedArchive()
		return
	default:
		ws.logger.Printf("pageHandler", r.RemoteAddr, nil, "just visiting")
		w.Write([]byte(fmt.Sprintf(UnlockPageHTML, "")))
		return
	}
}

// Start runs the web server and blocks until the server shuts down from a successful unlocking attempt.
func (ws *WebServer) Start() {
	ws.logger = &misc.Logger{
		ComponentName: "encarchive.WebServer",
		ComponentID:   strconv.Itoa(ws.Port),
	}
	ws.handlerMutex = new(sync.Mutex)
	// Page handler needs to know the size in order to prepare ramdisk
	stat, err := os.Stat(ws.ArchiveFilePath)
	if err != nil {
		ws.logger.Fatalf("Start", "", err, "failed to read archive file at %s", ws.ArchiveFilePath)
		return
	}
	ws.archiveFileSize = int(stat.Size())

	mux := http.NewServeMux()
	mux.HandleFunc(ws.URL, ws.pageHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	ws.server = &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%d", ws.Port), Handler: mux,
		ReadTimeout: HTTPTimeout, ReadHeaderTimeout: HTTPTimeout,
		WriteTimeout: HTTPTimeout, IdleTimeout: HTTPTimeout,
	}
	ws.logger.Printf("Start", "", nil, "will listen on TCP port %d", ws.Port)
	if err := ws.server.ListenAndServe(); err != nil && strings.Index(err.Error(), "closed") == -1 {
		ws.logger.Fatalf("Start", "", err, "failed to listen on TCP port")
		return
	}
	ws.logger.Printf("Start", "", nil, "web server has stopped")
	// Wait almost indefinitely (~5 years) because this is the main thread
	time.Sleep(5 * 365 * 24 * time.Hour)
}

/*
LaunchWithUnlockedArchive sleeps for a short moment, then shuts down the web server, and forks a process of laitos
program itself to launch using unlocked configuration and data from ramdisk.
*/
func (ws *WebServer) LaunchWithUnlockedArchive() {
	var fatalMsg string
	execPath, err := os.Executable()
	args := make([]string, 0, 8)
	var cmd *exec.Cmd
	// Give HTTP server a short moment to finish with pending connections
	shutdownTimeout, _ := context.WithTimeout(context.Background(), 10*time.Second)
	if err := ws.server.Shutdown(shutdownTimeout); err != nil {
		fatalMsg = fmt.Sprintf("failed to wait for HTTP server to shutdown - %v", err)
		goto fatalExit
	}
	if err != nil {
		fatalMsg = fmt.Sprintf("failed to determine executable path - %v", err)
		goto fatalExit
	}
	// Switch to the extracted, ramdisk directory for launching self.
	if err := os.Chdir(ws.ramdiskDir); err != nil {
		fatalMsg = fmt.Sprintf("failed to cd to %s - %v", ws.ramdiskDir, err)
		goto fatalExit
	}
	// Replicate the CLI arguments that were used to launch this launcher
	for i, arg := range os.Args {
		if i != 0 && arg != "-"+MagicArg {
			args = append(args, arg)
		}
	}
	ws.logger.Printf("LaunchWithUnlockedArchive", "", nil, "about to relaunch myself with args %v", args)
	cmd = exec.Command(execPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatalMsg = fmt.Sprintf("failed to launch self - %v", err)
		goto fatalExit
	}
	if err := cmd.Wait(); err != nil {
		fatalMsg = fmt.Sprintf("program exited abnormally - %v", err)
		goto fatalExit
	}
	ws.logger.Printf("LaunchWithUnlockedArchive", "", nil, "program has exited with clean status")
	// In both normal and abnormal paths, the ramdisk must be destroyed.
	DestroyRamdisk(ws.ramdiskDir)
	return
fatalExit:
	DestroyRamdisk(ws.ramdiskDir)
	ws.logger.Fatalf("LaunchWithUnlockedArchive", "", nil, fatalMsg)
}
