package handler

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/platform"
	"github.com/HouzuoGuo/laitos/remotevm"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	/*
		DefaultLinuxDistributionURL is the download URL of PuppyLinux, recommended for use with remote virtual machine controls.
		PuppyLinux is lightweight yet functional, it has been thoroughly tested with the remote virtual machine control feature.
	*/
	DefaultLinuxDistributionURL = "http://distro.ibiblio.org/puppylinux/puppy-fossa/fossapup64-9.5.iso"

	// HandleVirtualMachinePage is the web template of the virtual machine remote control.
	HandleVirtualMachinePage = `<html>
<head>
    <title>laitos remote virtual machine</title>
    <script type="text/javascript">
        <!--
        function set_pointer_coord(ev) {
            var pointer_x = ev.offsetX ? (ev.offsetX) : ev.pageX - document.getElementById('render').offsetLeft;
            var pointer_y = ev.offsetY ? (ev.offsetY) : ev.pageY - document.getElementById('render').offsetTop;
            document.getElementById('pointer_x').value = pointer_x;
            document.getElementById('pointer_y').value = pointer_y;
        };
        -->
    </script>
</head>
<body>
<form action="%s" method="post">
	<p>%s</p>
	<p>
		Info:
		<textarea rows="3" cols="80">%s</textarea>
	</p>
	<p>
		Virtual machine:
		<input type="submit" name="action" value="Refresh Screen"/>
		<input type="submit" name="action" value="Download OS"/>
		ISO URL:<input type="text" name="iso_url" value="%s"/>
		<input type="submit" name="action" value="Start"/>
		<input type="submit" name="action" value="Kill"/>
	</p>
	<p>
		Mouse:
		<input type="submit" name="action" value="LHold"/>
		<input type="submit" name="action" value="LRelease"/>
		<input type="submit" name="action" value="LDouble"/>
		<input type="submit" name="action" value="LClick"/>
		<input type="submit" name="action" value="RHold"/>
		<input type="submit" name="action" value="RRelease"/>
		<input type="submit" name="action" value="RDouble"/>
		<input type="submit" name="action" value="RClick"/>
		<input type="submit" name="action" value="Move To"/> X<input type="text" id="pointer_x" name="pointer_x" value="%d" size="2"/> Y<input type="text" id="pointer_y" name="pointer_y" value="%d" size="2"/>
	<p>
	<p>
		Keyboard:
		<input type="submit" name="action" value="Press Simultaneously"/>
		<input type="submit" name="action" value="Press One By One"/>
		Codes: <input type="text" name="press_keys" value="%s" size="50"/> (e.g. ctrl shift s)
	</p>
	<p>
		Useful key codes:
		f1-f12, 1-9, a-z, minus, equal, bracket_left, bracket_right, backslash<br/>
		semicolon, apostrophe, comma, dot, slash, esc, backspace, tab, ret, spc<br/>
		ctrl, shift, alt, up, down, left, right, home, end, pgup, pgdn, insert, delete<br/>
	</p>
	<p><img id="render" src="%s?rand=%d" alt="virtual machine screen" onclick="set_pointer_coord(event);"/></p>
</form>
</body>
</html>`
)

// HandleVirtualMachine is an HTTP handler that offers remote virtual machine controls, excluding the screenshot itself.
type HandleVirtualMachine struct {
	LocalUtilityPortNumber     int                             `json:"LocalUtilityPortNumber"`
	ScreenshotEndpoint         string                          `json:"-"`
	ScreenshotHandlerInstance  *HandleVirtualMachineScreenshot `json:"-"`
	VM                         *remotevm.VM                    `json:"-"`
	stripURLPrefixFromResponse string
	logger                     lalog.Logger
}

// Initialise internal state of the HTTP handler.
func (handler *HandleVirtualMachine) Initialise(logger lalog.Logger, _ *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	handler.logger = logger
	// Calculate the number of CPUs and amount of memory to be granted to virtual machine
	// Give the virtual machine half of the system CPUs
	numCPUs := (runtime.NumCPU() + 1) / 2
	// Give each CPU 384MB of memory, or in total up to 25% of system main memory to work with.
	memSizeMB := numCPUs * 384
	if _, totalKB := platform.GetSystemMemoryUsageKB(); totalKB > 0 {
		if quarterOfMainMB := totalKB / 1024 / 4; quarterOfMainMB > memSizeMB {
			memSizeMB = quarterOfMainMB
		}
	}

	// Create virtual machine instance with adequate amount of RAM and CPU
	handler.VM = &remotevm.VM{
		NumCPU:    numCPUs,
		MemSizeMB: memSizeMB,
		// The TCP port for interacting with emulator comes from user configuration input
		QMPPort: handler.LocalUtilityPortNumber,
	}
	if err := handler.VM.Initialise(); err != nil {
		return err
	}
	// Screenshots are taken from the same VM
	handler.ScreenshotHandlerInstance.VM = handler.VM
	handler.stripURLPrefixFromResponse = stripURLPrefixFromResponse
	return nil
}

/*
renderRemoteVMPage renders the HTML page that offers virtual machine control.
Virtual machine screenshot sits in a <img> tag, though the image data is served by a differe, dedicated handler.
*/
func (handler *HandleVirtualMachine) renderRemoteVMPage(requestURL string, err error, isoURL string, pointerX, pointerY int, pressKeys string) []byte {
	var errStr string
	if err != nil {
		errStr = err.Error()
	}
	return []byte(fmt.Sprintf(HandleVirtualMachinePage,
		requestURL, errStr, handler.VM.GetDebugOutput(),
		isoURL,
		pointerX, pointerY,
		pressKeys,
		strings.TrimPrefix(handler.ScreenshotEndpoint, handler.stripURLPrefixFromResponse), time.Now().UnixNano()))
}

// parseSubmission reads form action (button) and form text fields input.
func (handler *HandleVirtualMachine) parseSubmission(r *http.Request) (button, isoURL string, pointerX, pointerY int, pressKeys string) {
	button = r.FormValue("action")
	isoURL = r.FormValue("iso_url")
	pointerX, _ = strconv.Atoi(r.FormValue("pointer_x"))
	pointerY, _ = strconv.Atoi(r.FormValue("pointer_y"))
	pressKeys = r.FormValue("press_keys")
	return
}

// getISODownloadLocation returns the file system location where downloaded OS ISO file is kept,.
func (handler *HandleVirtualMachine) getISODownloadLocation() string {
	// Prefer to use user's home directory over temp directory so that it won't be deleted when laitos restarts.
	parentDir, _ := os.UserHomeDir()
	if parentDir == "" {
		parentDir = os.TempDir()
	}
	return path.Join(parentDir, ".laitos-remote-vm-iso-download.iso")
}

// Handle renders HTML page, reads user input from HTML form submission, and carries out corresponding VM control operations.
func (handler *HandleVirtualMachine) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	NoCache(w)
	if r.Method == http.MethodGet {
		// Display the web page. Suggest user to download the default Linux distribution.
		_, _ = w.Write(handler.renderRemoteVMPage(strings.TrimPrefix(r.RequestURI, handler.stripURLPrefixFromResponse), nil, DefaultLinuxDistributionURL, 0, 0, ""))
	} else if r.Method == http.MethodPost {
		// Handle buttons
		button, isoURL, pointerX, pointerY, pressKeys := handler.parseSubmission(r)
		var actionErr error
		switch button {
		case "Refresh Screen":
			// Simply re-render the page, including the screenshot. No extra action is required.
		case "Download OS":
			go func() {
				_ = handler.VM.DownloadISO(isoURL, handler.getISODownloadLocation())
			}()
			actionErr = errors.New(`Download is in progress, use "Refresh Screen" button to monitor the progress from Info output.`)
		case "Start":
			// Kill the older VM (if it exists) and then start a new VM
			handler.VM.Kill()
			if _, isoErr := os.Stat(handler.getISODownloadLocation()); os.IsNotExist(isoErr) {
				// If an ISO file does not yet exist, download the default Linux distribution.
				actionErr = errors.New(`Downloading Linux distribution, use "Refresh Screen" to monitor the progress from Info output, and then press "Start" again.`)
				go func() {
					_ = handler.VM.DownloadISO(isoURL, handler.getISODownloadLocation())
				}()
			} else {
				actionErr = handler.VM.Start(handler.getISODownloadLocation())
			}
		case "Kill":
			handler.VM.Kill()
		case "LHold":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
			if actionErr == nil {
				actionErr = handler.VM.HoldMouse(true, true)
			}
		case "LRelease":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
			if actionErr == nil {
				actionErr = handler.VM.HoldMouse(true, false)
			}
		case "LDouble":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
			if actionErr == nil {
				actionErr = handler.VM.DoubleClickMouse(true)
			}
		case "LClick":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
			if actionErr == nil {
				actionErr = handler.VM.ClickMouse(true)
			}
		case "RHold":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
			if actionErr == nil {
				actionErr = handler.VM.HoldMouse(false, true)
			}
		case "RRelease":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
			if actionErr == nil {
				actionErr = handler.VM.HoldMouse(false, false)
			}
		case "RDouble":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
			if actionErr == nil {
				actionErr = handler.VM.DoubleClickMouse(false)
			}
		case "RClick":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
			if actionErr == nil {
				actionErr = handler.VM.ClickMouse(false)
			}
		case "Move To":
			actionErr = handler.VM.MoveMouse(pointerX, pointerY)
		case "Press Simultaneously":
			keys := regexp.MustCompile(`[a-zA-Z0-9_]+`).FindAllString(pressKeys, -1)
			if len(keys) > 0 {
				actionErr = handler.VM.PressKeysSimultaneously(keys...)
			}
		case "Press One By One":
			keys := regexp.MustCompile(`[a-zA-Z0-9_]+`).FindAllString(pressKeys, -1)
			if len(keys) > 0 {
				actionErr = handler.VM.PressKeysOneByOne(keys...)
			}
		default:
			actionErr = fmt.Errorf("Unknown button action: %s", button)
		}
		_, _ = w.Write(handler.renderRemoteVMPage(strings.TrimPrefix(r.RequestURI, handler.stripURLPrefixFromResponse), actionErr, isoURL, pointerX, pointerY, pressKeys))
	}
}

// GetRateLimitFactor returns 3, which is at least 3 actions/second, more than sufficient for a virtual machine operator.
func (_ *HandleVirtualMachine) GetRateLimitFactor() int {
	return 3
}

// SelfTest is not applicable to this HTTP handler.
func (_ *HandleVirtualMachine) SelfTest() error {
	return nil
}

// HandleVirtualMachineScreenshot is an HTTP handler that takes a screenshot of remote virtual machine and serves it in JPEG.
type HandleVirtualMachineScreenshot struct {
	VM *remotevm.VM `json:"-"`
}

// Initialise is not applicable to this HTTP handler, as its internal
func (_ *HandleVirtualMachineScreenshot) Initialise(lalog.Logger, *toolbox.CommandProcessor, string) error {
	// Initialised by HandleVirtualMachine.Initialise
	return nil
}

// Handle takes a virtual machine screenshot and responds with JPEG image data completed with appropriate HTTP headers.
func (handler *HandleVirtualMachineScreenshot) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	// Store screenshot picture in a temporary file
	screenshot, err := ioutil.TempFile("", "laitos-handle-vm-screenshot")
	if err != nil {
		http.Error(w, "Failed to create temporary file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = screenshot.Close()
	defer os.Remove(screenshot.Name())
	if err := handler.VM.TakeScreenshot(screenshot.Name()); err != nil {
		http.Error(w, "Failed to create temporary file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jpegContent, err := ioutil.ReadFile(screenshot.Name())
	if err != nil {
		http.Error(w, "Failed to read screenshot file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(jpegContent)))
	_, _ = w.Write(jpegContent)
}

// GetRateLimitFactor returns 3, which is at least 3 screenshots/second, more than sufficient for a virtual machine operator.
func (_ *HandleVirtualMachineScreenshot) GetRateLimitFactor() int {
	return 3
}

// SelfTest is not applicable to this HTTP handler.
func (_ *HandleVirtualMachineScreenshot) SelfTest() error {
	return nil
}
