package browser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/httpclient"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	JSCodeTemplate = `var browser;

var b_redraw = function () {
    if (!browser) {
        return false;
    }
    browser.render('%s');
    return true;
};

var b_back = function () {
    if (!browser) {
        return false;
    }
    browser.goBack();
    return true;
};

var b_forward = function () {
    if (!browser) {
        return false;
    }
    browser.goForward();
    return true;
};

var b_reload = function () {
    if (!browser) {
        return false;
    }
    browser.reload();
    return true;
};

var b_goto = function (param) {
    if (!browser) {
        browser = require('webpage').create();
    }
    // hehehe screw user agent
    browser.settings.userAgent = param.user_agent;
    browser.viewportSize = {
        width: param.view_width,
        height: param.view_height
    };
    browser.onResourceError = function (err) {
        console.log('b_goto error: ' + err.errorString);
    };
    browser.open(param.page_url, function (result) {
        console.log('b_goto: ' + result);
    });
    return true;
};

var b_info = function () {
    var ret = '';
    if (browser) {
        ret = {
            'title': browser.evaluate(function () {
                return document.title;
            }),
            'page_url': browser.url
        };
    }
    return ret;
};

var b_pointer = function (param) {
    if (!browser) {
        return false;
    }
    browser.sendEvent(param.type, param.x, param.y, param.button);
    return true;
};

var b_type = function (param) {
    if (!browser) {
        return false;
    }
    if (param.key_string) {
        browser.sendEvent('keypress', param.key_string);
    } else {
        browser.sendEvent('keypress', parseInt(param.key_code));
    }
    return true;
};

var server = require('webserver').create().listen('127.0.0.1:%d', function (req, resp) {
    resp.statusCode = 200;
    resp.headers = {
        'Cache-Control': 'no-cache, no-store, must-revalidate',
        'Content-Type': 'application/json'
    };
    var ret = null;
    if (req.url === '/redraw') {
        ret = b_redraw();
    } else if (req.url === '/back') {
        ret = b_back();
    } else if (req.url === '/forward') {
        ret = b_forward();
    } else if (req.url === '/reload') {
        ret = b_reload();
    } else if (req.url === '/goto') {
        ret = b_goto(req.post);
    } else if (req.url === '/info') {
        ret = b_info();
    } else if (req.url === '/pointer') {
        ret = b_pointer(req.post);
    } else if (req.url === '/type') {
        ret = b_type(req.post);
    }
    console.log(req.method + ' ' + req.url + ' - ' + JSON.stringify(req.post) + ': ' + JSON.stringify(ret));
    resp.write(JSON.stringify(ret));
    resp.close();
    if (req.url === '/close') {
        phantom.exit();
    }
});
` // Template javascript code that runs on headless browser server
)

var TagCounter = int64(0) // Increment only counter that assigns each started browser its tag. Value 0 is an invalid tag.

// An instance of headless browser server that acts on commands received via HTTP.
type Renderer struct {
	PhantomJSExecPath  string        // Absolute or relative path to PhantomJS executable
	RenderImagePath    string        // Place to store rendered web page image
	Port               int           // Port number for headless server to listen for commands on
	AutoKillTimeoutSec int           // Process is unconditionally killed after the time elapses
	Tag                string        // Uniquely identifies this browser server after it is started
	DebugOutput        *bytes.Buffer // Store standard output and error from PhantomJS executable
	JSTmpFilePath      string        // Path to temporary file that stores PhantomJS server code
	JSProc             *exec.Cmd     // Headless server process
	JSProcMutex        *sync.Mutex   // Protect against concurrent access to server process
	Logger             global.Logger
}

// Produce javascript code for browser server and then launch its process in background.
func (instance *Renderer) Start() error {
	// Renderer is an internal API, hence its parameters are not validated before use.
	instance.JSProcMutex = new(sync.Mutex)
	instance.DebugOutput = new(bytes.Buffer)
	instance.Tag = strconv.FormatInt(atomic.AddInt64(&TagCounter, 1), 10)
	instance.Logger = global.Logger{ComponentID: fmt.Sprintf("%s-%s", time.Now().Format(time.Kitchen), instance.Tag), ComponentName: "Renderer"}
	// Store server javascript into a temporary file
	serverJS, err := ioutil.TempFile("", "laitos-browser")
	if err != nil {
		return fmt.Errorf("Renderer.Start: failed to create temporary file for PhantomJS code - %v", err)
	}
	if _, err := serverJS.Write([]byte(fmt.Sprintf(JSCodeTemplate, instance.RenderImagePath, instance.Port))); err != nil {
		return fmt.Errorf("Renderer.Start: failed to write PhantomJS server code - %v", err)
	} else if err := serverJS.Sync(); err != nil {
		return fmt.Errorf("Renderer.Start: failed to write PhantomJS server code - %v", err)
	} else if err := serverJS.Close(); err != nil {
		return fmt.Errorf("Renderer.Start: failed to write PhantomJS server code - %v", err)
	}
	// Start server process
	instance.JSProc = exec.Command(instance.PhantomJSExecPath, "--ssl-protocol=any", "--ignore-ssl-errors=yes", serverJS.Name())
	instance.JSProc.Stdout = instance.DebugOutput
	instance.JSProc.Stderr = instance.DebugOutput
	//browser.JSProc.Stdout = os.Stderr
	//browser.JSProc.Stderr = os.Stderr
	processErrChan := make(chan error, 1)
	go func() {
		if err := instance.JSProc.Start(); err != nil {
			processErrChan <- err
		}
	}()
	// Expect server process to remain running for at least a second for a successful start
	select {
	case err := <-processErrChan:
		return fmt.Errorf("Renderer.Start: PhantomJS process failed - %v", err)
	case <-time.After(1 * time.Second):
	}
	// Unconditionally kill the server process after a period of time
	go func() {
		select {
		case err := <-processErrChan:
			log.Printf("Renderer.Start: PhantomJS process has quit, status - %v", err)
		case <-time.After(time.Duration(instance.AutoKillTimeoutSec) * time.Second):
		}
		instance.Kill()
	}()
	// Keep knocking on the server port until it is open
	var portIsOpen bool
	for i := 0; i < 20; i++ {
		if _, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(instance.Port), 2*time.Second); err == nil {
			portIsOpen = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !portIsOpen {
		instance.Kill()
		return errors.New("Renderer.Start: port is not listening after multiple atempts")
	}
	return nil
}

// Return last N bytes of text from debug output buffer.
func (instance *Renderer) GetDebugOutput(lastNBytes int) string {
	all := instance.DebugOutput.Bytes()
	if len(all) > lastNBytes {
		return string(all[len(all)-lastNBytes:])
	} else {
		return string(all)
	}
}

// Send a control request via HTTP to the browser server, optionally deserialise the response into receiver.
func (instance *Renderer) SendRequest(actionName string, params map[string]interface{}, jsonReceiver interface{}) (err error) {
	body := url.Values{}
	if params != nil {
		for key, val := range params {
			body[key] = []string{fmt.Sprint(val)}
		}
	}
	resp, err := httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(body.Encode()),
	}, fmt.Sprintf("http://127.0.0.1:%d/%s", instance.Port, actionName))
	if err == nil {
		if resp.StatusCode/200 != 1 {
			err = fmt.Errorf("Renderer.SendRequest: HTTP failure - %v", string(resp.Body))
		}
		if jsonReceiver != nil {
			if jsonErr := json.Unmarshal(resp.Body, &jsonReceiver); jsonErr != nil {
				err = fmt.Errorf("Renderer.SendRequest: - %v", jsonErr)
			}
		}
	}
	instance.Logger.Printf("SendRequest", "", err, "%s(%s) - %s", actionName, body.Encode(), string(resp.Body))
	return
}

// Tell browser to render page and wait up to 3 seconds for render to finish.
func (instance *Renderer) RenderPage() error {
	if err := os.Remove(instance.RenderImagePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := instance.SendRequest("redraw", nil, nil); err != nil {
		return err
	}
	var fileSize int64
	var unchanging int
	for i := 0; i < 60; i++ {
		// See whether image file is already being written into
		if info, err := os.Stat(instance.RenderImagePath); err == nil && info.Size() > 0 {
			if fileSize == info.Size() {
				unchanging++
				if unchanging >= 4 {
					// If size looks identical to last four checks, the render is considered done.
					return nil
				}
			} else {
				// Size is changing, so render is not yet completed.
				unchanging = 0
				fileSize = info.Size()
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("Renderer.RenderPage: render is not completed")
}

// Kill browser server process and delete rendered web page image.
func (instance *Renderer) Kill() {
	if instance.JSProc != nil {
		instance.JSProcMutex.Lock()
		defer instance.JSProcMutex.Unlock()
		if err := os.Remove(instance.RenderImagePath); err != nil {
			instance.Logger.Warningf("Kill", "", err, "failed to delete rendered web page at \"%s\"", instance.RenderImagePath)
		}
		if err := instance.JSProc.Process.Kill(); err != nil {
			instance.Logger.Warningf("Kill", "", err, "failed to kill PhantomJS process")
		}
		instance.JSProc = nil
	}
}
