package remotevm

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

const (
	// QEMUExecutableName is the X86(64-bit) QEMU program's executable name, without the prefix path.
	QEMUExecutableName = "qemu-system-x86_64"
	// QMPCommandTimeoutSec is the number of seconds to wait for a QMP response before timing out.
	QMPCommandTimeoutSec = 30
	// AutoKillTimeoutSec is the number of seconds after which the emulator will be forcibly stopped.
	AutoKillTimeoutSec = 3600 * 24
)

/*
VM launches a virtual machine of lightweight Linux distribution via KVM (preferred) or QEMU (fall-back) and offers
remote mouse and keyboard control, as well as screenshot capability.
*/
type VM struct {
	NumCPU    int // NumCPU is the number of CPU cores allocated to emulator
	MemSizeMB int // MemSizeMB is the amount of memory allocated to emulator
	QMPPort   int // QMPPort is the TCP port number used for interacting with emulator

	emulatorExecutable  string
	emulatorCmd         *exec.Cmd
	emulatorDebugOutput *lalog.ByteLogWriter
	qmpClient           *textproto.Conn

	lastScreenWidth, lastScreenHeight int

	mutex  *sync.Mutex
	logger lalog.Logger
}

// Initialise internal states. It also determines the availability of KVM and QEMU on the host system.
func (vm *VM) Initialise() error {
	// Prefer to use the much-faster KVM. KVM requires root privilege
	if os.Getuid() == 0 {
		for _, prefixDir := range strings.Split(platform.CommonPATH, ":") {
			kvmPath := path.Join(prefixDir, "kvm")
			if _, err := os.Stat(kvmPath); err == nil {
				vm.emulatorExecutable = kvmPath
				break
			}
			qemuKVMPath := path.Join(prefixDir, "qemu-kvm")
			if _, err := os.Stat(qemuKVMPath); err == nil {
				vm.emulatorExecutable = qemuKVMPath
				break
			}
		}
	}
	// Look for regular QEMU if KVM isn't available
	if vm.emulatorExecutable == "" {
		for _, prefixDir := range strings.Split(platform.CommonPATH, ":") {
			qemuPath := path.Join(prefixDir, QEMUExecutableName)
			if _, err := os.Stat(qemuPath); err == nil {
				vm.emulatorExecutable = qemuPath
				break
			}
		}
	}
	// If neither KVM nor QEMU can be found in common PATH, then let OS do its best to look for QEMU.
	if vm.emulatorExecutable == "" {
		vm.emulatorExecutable = QEMUExecutableName
	}
	// Look for QEMU among program files of Windows
	if misc.HostIsWindows() {
		winQEMU := fmt.Sprintf(`C:\Program Files\qemu\%s.exe`, QEMUExecutableName)
		if _, err := os.Stat(winQEMU); err == nil {
			vm.emulatorExecutable = winQEMU
		}
	}

	vm.logger = lalog.Logger{
		ComponentName: "vm",
		ComponentID: []lalog.LoggerIDField{{
			Key:   path.Base(vm.emulatorExecutable),
			Value: fmt.Sprintf("%dC%dM", vm.NumCPU, vm.MemSizeMB),
		}},
	}
	// Keep the latest 1KB of emulator output for on-demand diagnosis. ISO download progress and QMP command execution result are also kept here.
	vm.emulatorDebugOutput = lalog.NewByteLogWriter(ioutil.Discard, 1024)
	vm.mutex = new(sync.Mutex)
	return nil
}

// DownloadISO downloads an ISO file from the input URL and saves it in a file. There is a hard limit of 15 minutes for the download operation to complete.
func (vm *VM) DownloadISO(isoURL string, destPath string) error {
	// No need to use a mutex.
	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Get(isoURL)
	fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: saving %s to %s, this may take a while.\n", isoURL, destPath)
	if err != nil {
		fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: download failed - %v\n", err)
		return fmt.Errorf("DownloadISO: failed to download %s - %w", isoURL, err)
	}
	defer resp.Body.Close()
	// Download the new ISO image into a temporary file
	tmpFile, err := ioutil.TempFile("", path.Base(destPath)+".tmp")
	if err != nil {
		fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: failed to create temporary file - %v\n", err)
		return fmt.Errorf("DownloadISO: failed to create temporary file - %w", err)
	}
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: download failed - %v\n", err)
		return fmt.Errorf("DownloadISO: failed to download %s - %w", isoURL, err)
	}
	if err := tmpFile.Close(); err != nil {
		fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: failed to save temporary file - %v\n", err)
		return fmt.Errorf("DownloadISO: failed to close temporary file %s - %w", tmpFile.Name(), err)
	}
	// Check ISO file access and file size
	stat, err := os.Stat(tmpFile.Name())
	if err != nil {
		fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: failed to read temporary file - %v\n", err)
		return fmt.Errorf("DownloadISO: failed to read temporary file %s - %w", tmpFile.Name(), err)
	}
	if stat.Size() < 8*1048576 {
		fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: ISO file seems too small (only %d MB)\n", stat.Size()/1048576)
		return fmt.Errorf("DownloadISO: ISO file seems too small (only %d MB)", stat.Size()/1048576)
	}
	// Move the new file in-place, possibly overwriting an existing ISO file.
	if err := os.Rename(tmpFile.Name(), destPath); err != nil {
		fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: failed to move downloaded ISO file to %s - %v\n", destPath, err)
		return fmt.Errorf("DownloadISO: failed to move downloaded ISO file to %s - %w", destPath, err)
	}
	fmt.Fprintf(vm.emulatorDebugOutput, "DownloadISO: successfully saved %s (%d MB) to %s\n", isoURL, stat.Size()/1048576, destPath)
	return nil
}

/*
Start the virtual machine. The function returns to the caller as soon as QEMU/KVM becomes ready to accept
commands. The emulator started is subjected to a time-out of 24-hours, after which it will be killed forcibly.
*/
func (vm *VM) Start(isoFilePath string) error {
	if _, err := os.Stat(isoFilePath); err != nil {
		return fmt.Errorf("VM.Start: failed to read OS ISO file \"%s\" - %v", isoFilePath, err)
	}

	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	// Prevent repeated startup of the same VM
	if vm.emulatorCmd != nil {
		return errors.New("VM.Start: already started")
	}
	vm.logger.Info("Start", isoFilePath, nil, "starting emulator %s, this may take a minute", vm.emulatorExecutable)
	fmt.Fprintf(vm.emulatorDebugOutput, "Starting emulator %s for ISO file %s, this may take a minute.\n", vm.emulatorExecutable, isoFilePath)
	emulatorProcErr := make(chan error, 1)
	vm.emulatorCmd = exec.Command(vm.emulatorExecutable,
		"-smp", strconv.Itoa(vm.NumCPU), "-m", fmt.Sprintf("%dM", vm.MemSizeMB),
		/*
			"nographic" tells emulator not to create a GUI window for interacting with VM. The emulator still gets a graphics card.
			For some reason, screenshots taken using "std" graphics are little shorter than actual VM graphics output; "vmware" graphics
			is not well supported by lightweight Linux distributions.
			The much older "cirrus" graphics card works the best.
		*/
		"-vga", "cirrus", "-nographic",
		/*
			Use a USB bus and a USB mouse ("tablet") for manipulating mouse pointer using absolute coordinates.
			Without a "tablet" mouse, we cannot position mouse pointer using absolute X&Y coordinates.
		*/
		"-usb", "-device", "usb-tablet",
		// Boot from CD which is an ISO file, usually that of a live Linux distribution.
		"-boot", "order=d", "-cdrom", isoFilePath,
		// Start command server
		"-qmp", fmt.Sprintf("tcp:127.0.0.1:%d,server,nowait", vm.QMPPort))
	vm.emulatorCmd.Stdout = vm.emulatorDebugOutput
	vm.emulatorCmd.Stderr = vm.emulatorDebugOutput
	// Start the emulator process in background
	go func() {
		emulatorCmd := vm.emulatorCmd
		if err := emulatorCmd.Start(); err != nil {
			emulatorProcErr <- err
			return
		}
		if err := emulatorCmd.Wait(); err != nil {
			emulatorProcErr <- err
		}
	}()
	// Connect and prepare QMP connection
	if err := vm.connectToQMP(); err != nil {
		vm.Kill()
		return fmt.Errorf("Start: failed to prepare QMP client - %w", err)
	}
	// Unconditionally kill the emulator after a period of time
	go func() {
		select {
		case err := <-emulatorProcErr:
			vm.logger.Info("Start", "", err, "emulator has quit")
		case <-time.After(AutoKillTimeoutSec * time.Second):
		}
		vm.Kill()
	}()
	fmt.Fprint(vm.emulatorDebugOutput, "Emulator started successfully.\n", vm.emulatorExecutable, isoFilePath)
	return nil
}

/*
connectToQMP is an internal function that initialises a QMP client connection and prepares it with initial mandatory command exchange.
The function tolerates temporary connection failures.
*/
func (vm *VM) connectToQMP() error {
	var qmpConn net.Conn
	var connErr error
	// Give the server 10 seconds to start
	for i := 0; i < 10*10; i++ {
		qmpConn, connErr = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", vm.QMPPort), 1*time.Second)
		if connErr == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if connErr != nil {
		vm.Kill()
		return connErr
	}
	// Absorb the greeting JSON message
	vm.qmpClient = textproto.NewConn(qmpConn)
	greeting, err := vm.qmpClient.ReadLine()
	if err != nil || !strings.Contains(greeting, "QMP") {
		fmt.Fprintf(vm.emulatorDebugOutput, "QMP: missing protocol greeting -  %v %s\n", err, greeting)
		vm.Kill()
		return fmt.Errorf("QMP did not send greeting - %w %s", err, greeting)
	}
	// Exchange the mandatory initialisation command
	if err := vm.qmpClient.PrintfLine(`{"execute":"qmp_capabilities"}`); err != nil {
		return fmt.Errorf("Failed to exchange initialisation QMP command - %w", err)
	}
	if _, err := vm.qmpClient.ReadLine(); err != nil {
		return fmt.Errorf("Failed to exchange initialisation QMP command - %w", err)
	}
	return nil
}

// Kill the emulator.
func (vm *VM) Kill() {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	if client := vm.qmpClient; client != nil {
		_ = client.Close()
	}
	vm.qmpClient = nil
	if emulatorCmd := vm.emulatorCmd; emulatorCmd != nil {
		if emulatorProc := emulatorCmd.Process; emulatorProc != nil {
			vm.logger.Info("Kill", "", nil, "killing process PID %d", emulatorProc.Pid)
			if !platform.KillProcess(emulatorProc) {
				vm.logger.Warning("Kill", "", nil, "failed to kill emulator process")
			}
		}
	}
	vm.emulatorCmd = nil
}

// GetDebugOutput returns the QEMU/KVM emulator output along with recent QMP command and responses.
func (vm *VM) GetDebugOutput() string {
	if vm.emulatorDebugOutput != nil {
		return string(vm.emulatorDebugOutput.Retrieve(true))
	}
	return ""
}

/*
TakeScreenshot takes a screenshot of the emulator video display, the screenshot image format is JPEG.
The function also updates the screen total resolution tracked internally for calculating mouse movement coordinates.
*/
func (vm *VM) TakeScreenshot(outputFileName string) error {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	// Create a temporary file to store the screenshot output
	tmpFile, err := ioutil.TempFile("", "laitos-vm-take-screenshot*.ppm")
	if err != nil {
		return err
	}
	_ = tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	// Ask QEMU to take the screenshot
	_, err = vm.executeQMP(map[string]interface{}{
		"execute": "screendump",
		"arguments": map[string]interface{}{
			"filename": tmpFile.Name(),
		},
	})
	if err != nil {
		return err
	}
	// QEMU takes a short while to finish taking the screenshot even if the positive response comes instantenously
	var fileSize int64
	var unchanging int
anticiateGrowingFile:
	for i := 0; i < 60; i++ {
		if info, err := os.Stat(tmpFile.Name()); err == nil && info.Size() > 0 {
			if fileSize == info.Size() {
				// The screenshot is complete if the file size looks identical for 4 consecutive checks
				unchanging++
				if unchanging >= 4 {
					break anticiateGrowingFile
				}
			} else {
				// File size is changing so QEMU is still busy taking the screenshot
				unchanging = 0
				fileSize = info.Size()
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if fileSize == 0 {
		return errors.New("VM.TakeScreenshot: screenshot command was sent, however the result screenshot file is empty.")
	}
	// Decode screenshot in PPM format
	ppmFile, err := os.Open(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("VM.TakeScreenshot: failed to open screenshot file - %w", err)
	}
	ppmImage, err := readPPM(ppmFile)
	if err != nil {
		return fmt.Errorf("VM.TakeScreenshot: failed to decode screenshot file - %w", err)
	}
	_ = ppmFile.Close()
	// Memorise the latest screen resolution to help calculating mouse movement coordinates
	vm.lastScreenWidth = ppmImage.Bounds().Size().X
	vm.lastScreenHeight = ppmImage.Bounds().Size().Y
	// Encode the screenshot in JPEG and save to output file
	jpegFile, err := os.OpenFile(outputFileName, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("VM.TakeScreenshot: failed to create screenshot file - %w", err)
	}
	defer func() {
		_ = jpegFile.Close()
	}()
	if err := jpeg.Encode(jpegFile, ppmImage, nil); err != nil {
		return fmt.Errorf("VM.TakeScreenshot: failed to save screenshot file - %w", err)
	}
	return nil
}

/*
MoveMouse moves the mouse cursor to the input location.
Prior to calling this function the caller should have quite recently taken a screenshot of the VM, because
the resolution of the VM screen is internally memorised to help with calculating mouse movement coordinates.
*/
func (vm *VM) MoveMouse(x, y int) error {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	/*
		Be aware that few live Linux distributions do not work with QEMU mouse input, such as TinyCore.

		Calculating mouse pointer coordinates is subjected to a scaling effect, a complicated formular
		unique to each kind of emulated graphics card. For PuppyLinux running on "cirrus" graphics, here
		are the findings:

		- At 1024x768, to position mouse at X=100, ask QEMU for X=100*32.
		  To position mouse at Y=100, ask QEMU for Y=100*42.68.
		- At 800x600, to position mouse at X=600, asking QEMU for X=600*32 causes mouse to miss X=600 and ends up at X=470 instead.
		  To position mouse at Y=600, asking QEMU for Y=600*42.68 causes mouse to miss Y=600 and ends up at Y=470 instead.

			Therefore, to position mouse at (X,Y) for screen resolution of W*H, ask QEMU for:
			X*(32*(1/(W/1024))), Y*(42.68*(1/(H/768))).
	*/
	_, err := vm.executeQMP(map[string]interface{}{
		"execute": "input-send-event",
		"arguments": map[string]interface{}{
			"events": []interface{}{
				map[string]interface{}{
					"type": "abs",
					"data": map[string]interface{}{
						"axis":  "x",
						"value": int(float64(x) * (32 * (1 / (float64(vm.lastScreenWidth) / 1024)))),
					},
				},
				map[string]interface{}{
					"type": "abs",
					"data": map[string]interface{}{
						"axis":  "y",
						"value": int(float64(y) * (42.68 * (1 / (float64(vm.lastScreenHeight) / 768)))),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	return nil
}

/*
ClickKeyboard pushes and releases the keys given in the input sequence all at once.
Keys are identified by "QCode", which is a string that indicates key's name.
E.g. in order to type the @ symbol, first configure the OS running inside VM to use the
US keyboard layout, and then send codes ["shift", "2"].

QEMU developers have made it very challenging to find the comprehensive list of QCodes,
but a partial list can be found at: https://en.wikibooks.org/wiki/QEMU/Monitor#sendkey_keys
*/
func (vm *VM) ClickKeyboard(qKeyCodes ...string) error {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	keys := make([]interface{}, len(qKeyCodes))
	for i, code := range qKeyCodes {
		keys[i] = map[string]interface{}{
			"type": "qcode",
			"data": code,
		}
	}
	_, err := vm.executeQMP(map[string]interface{}{
		"execute": "send-key",
		"arguments": map[string]interface{}{
			"keys": keys,
		},
	})
	if err != nil {
		return err
	}
	return nil
}

// HoldButton holds down or releases the left or right mouse button.
func (vm *VM) HoldMouse(leftButton, holdDown bool) error {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	button := "left"
	if !leftButton {
		button = "right"
	}
	_, err := vm.executeQMP(map[string]interface{}{
		"execute": "input-send-event",
		"arguments": map[string]interface{}{
			"events": []interface{}{
				map[string]interface{}{
					"type": "btn",
					"data": map[string]interface{}{
						"down":   holdDown,
						"button": button,
					},
				},
			},
		},
	})
	return err
}

// ClickMouse makes a 100 milliseconds long mouse click with either the left button or right mouse button.
func (vm *VM) ClickMouse(leftButton bool) error {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	button := "left"
	if !leftButton {
		button = "right"
	}
	// true state means push, false state means release.
	for _, state := range []bool{true, false} {
		_, err := vm.executeQMP(map[string]interface{}{
			"execute": "input-send-event",
			"arguments": map[string]interface{}{
				"events": []interface{}{
					map[string]interface{}{
						"type": "btn",
						"data": map[string]interface{}{
							"down":   state,
							"button": button,
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// DoubleClickMouse makes a double click with either left or right mouse button in 200 milliseconds.
func (vm *VM) DoubleClickMouse(leftButton bool) error {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	button := "left"
	if !leftButton {
		button = "right"
	}
	// true state means push, false state means release.
	for _, state := range []bool{true, false, true, false} {
		_, err := vm.executeQMP(map[string]interface{}{
			"execute": "input-send-event",
			"arguments": map[string]interface{}{
				"events": []interface{}{
					map[string]interface{}{
						"type": "btn",
						"data": map[string]interface{}{
							"down":   state,
							"button": button,
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

/*
executeQMP is an internal function that serialises the input QMP command and sends it to the emulator, and then awaits
emulator's response.
For the simplicity of implementation, each command makes a new TCP connection to the emulator's TCP server.
*/
func (vm *VM) executeQMP(in interface{}) (resp string, err error) {
	if vm.qmpClient == nil {
		return "", errors.New("QMP client is not initialised yet")
	}
	// Serialise incoming command
	req, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(vm.emulatorDebugOutput, "Debug: request - %s\n", string(req))
	// Send the input command
	if err := vm.qmpClient.PrintfLine(strings.ReplaceAll(string(req), "%", "%%")); err != nil {
		fmt.Fprintf(vm.emulatorDebugOutput, "Error: failed to send command -  %v %s\n", err, string(resp))
		return "", err
	}
	// Read the command response. The QMP responses are most often useless.
	resp, err = vm.qmpClient.ReadLine()
	fmt.Fprintf(vm.emulatorDebugOutput, "Debug: response - %v %s\n", err, string(resp))
	if err != nil {
		return
	}
	if !strings.Contains(resp, "return") {
		fmt.Fprintf(vm.emulatorDebugOutput, "Error: likely protocol error response - %v %s\n", err, string(resp))
		err = fmt.Errorf("executeQMP: likely protocol error response - %s", string(resp))
	}
	return
}
