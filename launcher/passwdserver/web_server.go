package passwdserver

import (
	"context"
	"fmt"
	"github.com/HouzuoGuo/laitos/launcher"
	"github.com/HouzuoGuo/laitos/launcher/encarchive"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"net"
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
	usedRoot, freeRoot, totalRoot := misc.GetRootDiskUsageKB()
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
	Port            int    // Port is the TCP port to listen on.
	URL             string // URL is the secretive URL that serves the unlock page. The URL must include leading slash.
	ArchiveFilePath string // ArchiveFilePath is the absolute or relative path to encrypted archive file.

	server           *http.Server // server is the HTTP server after it is started.
	archiveFileSize  int          // archiveFileSize is the size of the archive file, it is set when web server starts.
	extractedDataDir string       // extractedDataDir is destination location into which encrypted program data is extracted.
	handlerMutex     *sync.Mutex  // handlerMutex prevents concurrent unlocking attempts from being made at once.
	alreadyUnlocked  bool         // alreadyUnlocked is set to true after a successful unlocking attempt has been made

	logger misc.Logger
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
		/*
			By default, archive is extracted into ramdisk and does not touch disk storage. Doing so requires a Linux
			host and root privilege.
			Ramdisk size in MB = archive size (unencrypted archive) + archive size (extracted files) + 8 (contingency)
		*/
		ws.extractedDataDir, err = encarchive.MakeRamdisk(ws.archiveFileSize/1048576*2 + 8)
		if err != nil {
			/*
				Create a plain directory to hold decrypted program data, this is the less secure fallback method,
				because file recovery operation will reveal the decrypted program data. This fallback helps to launch
				encrypted archive in the absence of root privielge (e.g. AWS Fargate), or in case host is Windows or
				MacOS.
			*/
			ws.logger.Warning("pageHandler", r.RemoteAddr, nil, "host does not support ramdisk, hence trying insecure method to extract data")
			ws.extractedDataDir, err = encarchive.MakePlainDestDir()
		}
		if err != nil {
			w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, err.Error())))
			return
		}
		// Create a temporary file to hold the decrypted program archive
		tmpFile, err := ioutil.TempFile(ws.extractedDataDir, "launcher-extract-temp-file")
		if err != nil {
			w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, err.Error())))
			return
		}
		defer tmpFile.Close()
		defer os.Remove(tmpFile.Name())
		// Extract program archive
		if err := encarchive.Extract(ws.ArchiveFilePath, tmpFile.Name(), ws.extractedDataDir, []byte(strings.TrimSpace(r.FormValue("password")))); err != nil {
			encarchive.DestroyRamdisk(ws.extractedDataDir)
			w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, err.Error())))
			return
		}
		// Success!
		w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, "success")))
		ws.alreadyUnlocked = true
		// A short moment later, the function will launch laitos supervisor along with daemons.
		go ws.LaunchMainProgram()
		return
	default:
		ws.logger.Info("pageHandler", r.RemoteAddr, nil, "just visiting")
		w.Write([]byte(fmt.Sprintf(PageHTML, GetSysInfoText(), r.RequestURI, "")))
		return
	}
}

// Start runs the web server and blocks until the server shuts down from a successful unlocking attempt.
func (ws *WebServer) Start() error {
	ws.logger = misc.Logger{
		ComponentName: "passwdserver.WebServer",
		ComponentID:   []misc.LoggerIDField{{"Port", ws.Port}},
	}
	ws.handlerMutex = new(sync.Mutex)
	// Page handler needs to know the size in order to prepare ramdisk
	stat, err := os.Stat(ws.ArchiveFilePath)
	if err != nil {
		ws.logger.Warning("Start", "", err, "failed to read archive file at %s", ws.ArchiveFilePath)
		return err
	}
	ws.archiveFileSize = int(stat.Size())

	mux := http.NewServeMux()
	// Visitor must visit the pre-configured URL for a meaningful response
	mux.HandleFunc(ws.URL, ws.pageHandler)
	// All other URLs simply render an empty page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	})

	/*
		If a previously launched laitos was killed by user, systemd, or supervisord, it would not have a chance to
		clean up after its own ramdisk. Therefore, free them up for this new launch.
	*/
	encarchive.TryDestroyAllRamdisks()
	encarchive.TryDestroyAllPlainDestDirs()

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
func (ws *WebServer) LaunchMainProgram() {
	var fatalMsg string
	var err error
	var executablePath string
	// Replicate the CLI flagsNoExec that were used to launch this password web server.
	flagsNoExec := make([]string, len(os.Args))
	copy(flagsNoExec, os.Args[1:])
	var cmd *exec.Cmd
	// Web server will take several seconds to finish with pending IO before shutting down
	if err := ws.Shutdown(); err != nil {
		fatalMsg = fmt.Sprintf("failed to shut down web server - %v", err)
		goto fatalExit
	}
	// Determine path to my program
	executablePath, err = os.Executable()
	if err != nil {
		fatalMsg = fmt.Sprintf("failed to determine path to this program executable - %v", err)
		goto fatalExit
	}
	// Switch to the ramdisk directory full of decrypted data for launching supervisor and daemons
	if err := os.Chdir(ws.extractedDataDir); err != nil {
		fatalMsg = fmt.Sprintf("failed to cd to %s - %v", ws.extractedDataDir, err)
		goto fatalExit
	}
	// Remove CLI flags that were used to launch the web server from the flags used to launch laitos main program
	flagsNoExec = launcher.RemoveFromFlags(func(s string) bool {
		return strings.HasPrefix(s, "-"+CLIFlag)
	}, flagsNoExec)
	ws.logger.Info("LaunchMainProgram", "", nil, "about to launch with CLI flagsNoExec %v", flagsNoExec)
	cmd = exec.Command(executablePath, flagsNoExec...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatalMsg = fmt.Sprintf("failed to launch main program - %v", err)
		goto fatalExit
	}
	if err := cmd.Wait(); err != nil {
		fatalMsg = fmt.Sprintf("main program has abnormally exited due to - %v", err)
		goto fatalExit
	}
	ws.logger.Info("LaunchMainProgram", "", nil, "main program has exited cleanly")
	// In both normal and abnormal paths, the ramdisk/temporary directory must be destroyed.
	encarchive.DestroyRamdisk(ws.extractedDataDir)
	encarchive.DestoryPlainDestDir(ws.extractedDataDir)
	return
fatalExit:
	encarchive.DestroyRamdisk(ws.extractedDataDir)
	encarchive.DestoryPlainDestDir(ws.extractedDataDir)
	ws.logger.Abort("LaunchMainProgram", "", nil, fatalMsg)
}
