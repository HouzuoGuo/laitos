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
	JSCodeTemplate = `try {
    var browser; // the browser page instance after very first URL is visited

    // ============== ACTIONS COMMON TO INTERACTIVE AND LINE-ORIENTED INTERFACE ==========

    // Re-render page screenshot.
    var b_redraw = function () {
        if (!browser) {
            return false;
        }
        browser.render('%s');
        return true;
    };

    // Navigate back.
    var b_back = function () {
        if (!browser) {
            return false;
        }
        b_lo_reset();
        browser.goBack();
        return true;
    };

    // Navigate forward.
    var b_forward = function () {
        if (!browser) {
            return false;
        }
        b_lo_reset();
        browser.goForward();
        return true;
    };

    // Reload the current URL (refresh).
    var b_reload = function () {
        if (!browser) {
            return false;
        }
        b_lo_reset();
        browser.reload();
        return true;
    };

    // Go to a new URL.
    var b_goto = function (param) {
        if (!browser) {
            browser = require('webpage').create();
            browser.onConsoleMessage = function (msg, line_num, src_id) {
                console.log("PAGE CONSOLE: " + msg);
            };
        }
        b_lo_reset();
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

    // Retrieve title and URL of the current page.
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

    // Move mouse pointer to a coordinate and optionally click a button.
    var b_pointer = function (param) {
        if (!browser) {
            return false;
        }
        browser.sendEvent(param.type, param.x, param.y, param.button);
        return true;
    };

    // Type keys into the currently focused element.
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

    // Run a web server that receives commands from HTTP clients.
    var server = require('webserver').create().listen('127.0.0.1:%d', function (req, resp) {
        resp.statusCode = 200;
        resp.headers = {
            'Cache-Control': 'no-cache, no-store, must-revalidate',
            'Content-Type': 'application/json'
        };
        var ret = null;
        if (req.url === '/redraw') {
            // curl -X POST 'localhost:12345/redraw'
            ret = b_redraw();
        } else if (req.url === '/back') {
            ret = b_back();
        } else if (req.url === '/forward') {
            ret = b_forward();
        } else if (req.url === '/reload') {
            ret = b_reload();
        } else if (req.url === '/goto') {
            // curl -X POST --data 'user_agent=user_agent=Mozilla%2F5.0%20(Windows%20NT%2010.0%3B%20Win64%3B%20x64)%20AppleWebKit%2F537.36%20(KHTML%2C%20like%20Gecko)%20Chrome%2F59.0.3071.115%20Safari%2F537.36&view_width=1024&view_height=1024&page_url=https%3A%2F%2Fgoogle.com' 'localhost:12345/goto'
            ret = b_goto(req.post);
        } else if (req.url === '/info') {
            // curl -X POST 'localhost:12345/info'
            ret = b_info();
        } else if (req.url === '/pointer') {
            // curl -X POST --data 'type=click&x=111&y=222&button=left' 'localhost:12345/type'
            ret = b_pointer(req.post);
        } else if (req.url === '/type') {
            // curl -X POST --data 'key_string=test123' 'localhost:12345/type'
            // (16777221 is enter key)
            // curl -X POST --data 'key_code=16777221' 'localhost:12345/type'
            ret = b_type(req.post);
        } else if (req.url === '/lo_reset') {
            // curl -X POST 'localhost:12345/lo_reset'
            ret = b_lo_reset();
        } else if (req.url === '/lo_next') {
            // curl -X POST 'localhost:12345/lo_next'
            ret = b_lo_next()
        } else if (req.url === '/lo_prev') {
            // curl -X POST 'localhost:12345/lo_prev'
            ret = b_lo_prev()
        } else if (req.url === '/lo_next_n') {
            // curl -X POST --data 'n=10' 'localhost:12345/lo_next_n'
            ret = b_lo_next_n(req.post);
        } else if (req.url === '/lo_pointer') {
            // curl -X POST --data 'type=click&button=left' 'localhost:12345/lo_pointer'
            ret = b_lo_pointer(req.post);
        } else if (req.url === '/lo_set_val') {
            // curl -X POST --data 'value=ABCDEFG' 'localhost:12345/lo_set_val'
            ret = b_lo_set_val(req.post);
        }
        console.log(req.method + ' ' + req.url + ' - ' + JSON.stringify(req.post) + ': ' + JSON.stringify(ret));
        resp.write(JSON.stringify(ret));
        resp.close();
        if (req.url === '/close') {
            phantom.exit();
        }
    });

    // ===================== ONLY FOR LINE-ORIENTED INTERFACE =================

    // The very previous element and its own previous/next element that were navigated into.
    var exact_info = null, before_info = null, after_info = null;

    // Put a string into double quotes.
    var quote_str = function (str) {
        return JSON.stringify(str);
    };

    // Return a string-encoded function body that store 4 element parameters into window object.
    var elem_info_to_stmt = function (elem_info) {
        return "function(){" +
            "window.laitos_pjs_tag = " + quote_str(elem_info === null ? '' : elem_info['tag']) + ";" +
            "window.laitos_pjs_id  = " + quote_str(elem_info === null ? '' : elem_info['id']) + ";" +
            "window.laitos_pjs_name = " + quote_str(elem_info === null ? '' : elem_info['name']) + ";" +
            "window.laitos_pjs_inner = " + quote_str(elem_info === null ? '' : elem_info['inner']) + ";" +
            "window.laitos_pjs_stop_at_first = " + (elem_info === null ? 'true' : 'false') + ";" +
            "}";
    };

    // Install several functions that help line-oriented browsing into window object.
    var lo_install_func = function () {
        // Look for an element, and return brief details of the element along with its previous and next element. Give the exact match the focus.
        window.laitos_pjs_find_before_after = function (tag, id, name, inner) {
            var before = null, exact = null, after = null, stop_next = false;
            laitos_pjs_walk(document.documentElement, function (elem) {
                if (!elem) {
                    return true;
                }
                var height = elem.offsetHeight,
                    width = elem.offsetWidth,
                    elem_inner = elem.innerHTML;
                // Only consider elements that are at least 9 square pixels large and content does not look exceedingly long
                if (height > 3 && width > 3 && elem_inner && elem_inner.length < 1000) {
                    if (stop_next) {
                        after = elem;
                        return false;
                    }
                    if (elem.tagName === tag && elem.id === id && elem.name === name && elem_inner === inner || laitos_pjs_stop_at_first) {
                        exact = elem;
                        window.laitos_pjs_current_elem = elem;
                        elem.focus();
                        stop_next = true;
                    } else {
                        before = elem;
                    }
                }
                return true;
            });
            return [
                before === null ? null : laitos_pjs_elem_to_obj(before),
                exact === null ? null : laitos_pjs_elem_to_obj(exact),
                after === null ? null : laitos_pjs_elem_to_obj(after)
            ];
        };

        // Turn a DOM element into an object that describes several of its details.
        window.laitos_pjs_elem_to_obj = function (elem) {
            return {
                "tag": elem.tagName,
                "id": elem.id,
                "name": elem.name,
                "value": elem.value,
                "inner": elem.innerHTML
            };
        };

        // Walk through DOM elements.
        window.laitos_pjs_walk = function (elem, walk_fun) {
            if (!elem) {
                // Continue
                return true;
            }
            for (var child = elem.childNodes, t = 0; t < child.length; t++) {
                if (!laitos_pjs_walk(child[t], walk_fun)) {
                    return false;
                }
            }
            return walk_fun(elem);
        };

        // Find elements that are immediately adjacent to the one described in parameters. Give the very last one to focus.
        window.laitos_pjs_find_after = function (tag, id, name, inner, num) {
            var ret = [], matched = false;
            laitos_pjs_walk(document.documentElement, function (elem) {
                if (!elem) {
                    return true;
                }
                var height = elem.offsetHeight,
                    width = elem.offsetWidth,
                    elem_inner = elem.innerHTML;
                // Only consider elements that are at least 9 square pixels large and content does not look exceedingly long
                if (height > 3 && width > 3 && elem_inner && elem_inner.length < 1000) {
                    if (elem.tagName === tag && elem.id === id && elem.name === name && elem_inner === inner) {
                        matched = true;
                        return true;
                    }
                    if (matched) {
                        if (ret.length < num) {
                            window.laitos_pjs_current_elem = elem;
                            elem.focus();
                            ret.push(laitos_pjs_elem_to_obj(elem));
                        } else {
                            return false;
                        }
                    }
                }
                return true;
            });
            return ret;
        };
    };

    // Reset recorded element information so that next DOM navigation will find the first element on page.
    var b_lo_reset = function () {
        before_info = null;
        exact_info = null;
        after_info = null;
    };

    // PhantomJS has a weird bug, if in page context a null value is returned to phantomJS caller, the value turns into an empty string.
    var empty_str_to_null = function (array) {
        for (var i = 0; i < array.length; i++) {
            if (array[i] === "") {
                array[i] = null;
            }
        }
        return array;
    };

    // Navigate to the next element.
    var b_lo_next = function () {
        if (!browser) {
            return false;
        }
        browser.evaluate(lo_install_func);
        if (exact_info === null) {
            console.log('b_lo_next: visit the first element');
            // Go to the first element, null parameter value will turn on laitos_pjs_stop_at_first.
            browser.evaluateJavaScript(elem_info_to_stmt(exact_info));
            // Invoke the search function using parameters stored in window object
        } else {
            if (after_info === null) {
                // If already at last element, do not go further next.
                console.log('b_lo_next: already at last element');
                browser.evaluateJavaScript(elem_info_to_stmt(exact_info));
            } else {
                // Go to the next element
                console.log('b_lo_next: visit the next element');
                browser.evaluateJavaScript(elem_info_to_stmt(after_info));

            }
        }
        var ret = empty_str_to_null(browser.evaluate(function () {
            return laitos_pjs_find_before_after(laitos_pjs_tag, laitos_pjs_id, laitos_pjs_name, laitos_pjs_inner);
        }));
        before_info = ret[0];
        exact_info = ret[1];
        after_info = ret[2];
        return ret;
    };

    // Navigate to the previous element.
    var b_lo_prev = function () {
        if (!browser) {
            return false;
        }
        browser.evaluate(lo_install_func);

        // If before_info is null, it will naturally visit the first element of the page.
        browser.evaluateJavaScript(elem_info_to_stmt(before_info));
        var ret = empty_str_to_null(browser.evaluate(function () {
            return laitos_pjs_find_before_after(laitos_pjs_tag, laitos_pjs_id, laitos_pjs_name, laitos_pjs_inner);
        }));

        before_info = ret[0];
        exact_info = ret[1];
        after_info = ret[2];
        return ret;
    };

    // Jump forward for a designated number of elements, and return information of elements seen on the way.
    var b_lo_next_n = function (param) {
        if (!browser) {
            return false;
        }
        browser.evaluate(lo_install_func);
        // If no element has ever been navigated into, go to the first element.
        if (exact_info === null) {
            b_lo_next();
        }
        browser.evaluateJavaScript(elem_info_to_stmt(exact_info));
        browser.evaluateJavaScript("function(){window.laitos_pjs_next_n=" + param.n + ";}");

        var ret = empty_str_to_null(browser.evaluate(function () {
            return laitos_pjs_find_after(laitos_pjs_tag, laitos_pjs_id, laitos_pjs_name, laitos_pjs_inner, laitos_pjs_next_n);
        }));

        if (ret.length > 0) {
            before_info = exact_info;
            // Intentionally set both exact and after element information to that belonging to very last element
            exact_info = ret[ret.length - 1];
            after_info = ret[ret.length - 1];
        }
        return ret;
    };

    // Send a pointer event (left/right click) to the page.
    var b_lo_pointer = function (param) {
        if (!browser) {
            return false;
        }
        var x = browser.evaluate(function () {
            if (!laitos_pjs_current_elem) {
                return false;
            }
            return laitos_pjs_current_elem.getBoundingClientRect().left + window.scrollX;
        });
        var y = browser.evaluate(function () {
            if (!laitos_pjs_current_elem) {
                return false;
            }
            return laitos_pjs_current_elem.getBoundingClientRect().top + window.scrollY;
        });
        if (x === false || y === false) {
            return false;
        }
        // Instead of pointing exactly on its boarder, point a bit into the element.
        return b_pointer({
            'type': param['type'],
            'x': x + 1,
            'y': y + 1,
            'button': param['button']
        });
    };

    // Set a value to the currently focused element.
    var b_lo_set_val = function (param) {
        if (!browser) {
            return false;
        }
        browser.evaluate(lo_install_func);
        browser.evaluateJavaScript("function(){window.laitos_pjs_set_value_to=" + JSON.stringify(param.value) + ";}");

        // Give the currently focused element a new value.
        return browser.evaluate(function () {
            if (!laitos_pjs_current_elem) {
                return false;
            }
            return laitos_pjs_current_elem.getBoundingClientRect().left + window.scrollX;
        });
    };

} catch
    (err) {
    var msg = "\nPhantomJS Javascript Exception";
    msg += "\nError: " + err.toString();
    for (var p in err) {
        msg += "\n" + p.toUpperCase() + ": " + ex[p];
    }
    console.log(msg);
}` // Template javascript code that runs on headless browser server

	// GoodUserAgent is the recommended user agent string for rendering all pages
	GoodUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"
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
	Index              int           // Index is the instance number assigned by renderer lifecycle management.
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
	//instance.JSProc.Stdout = os.Stderr
	//instance.JSProc.Stderr = os.Stderr
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

// GoBack navigates browser backward in history.
func (instance *Renderer) GoBack() error {
	return instance.SendRequest("back", nil, nil)
}

// GoForward navigates browser forward in history.
func (instance *Renderer) GoForward() error {
	return instance.SendRequest("forward", nil, nil)
}

// Reload reloads the current page.
func (instance *Renderer) Reload() error {
	return instance.SendRequest("reload", nil, nil)
}

// GoTo navigates to a new URL.
func (instance *Renderer) GoTo(userAgent, pageURL string, width, height int) error {
	var result bool
	err := instance.SendRequest("goto", map[string]interface{}{
		"user_agent":  userAgent,
		"page_url":    pageURL,
		"view_width":  width,
		"view_height": height,
	}, &result)
	if err != nil {
		return err
	}
	if !result {
		return errors.New("Renderer.GoTo: result is false")
	}
	return nil
}

const (
	PointerTypeClick   = "click" // PointerTypeClick is the phantomJS mouse action for clicking.
	PointerTypeMove    = "move"  // PointerTypeClick is the phantomJS mouse action for moving pointer.
	PointerButtonLeft  = "left"  // PointerTypeClick is the phantomJS left mouse button.
	PointerButtonRight = "right" // PointerTypeClick is the phantomJS right mouse button.
)

// Pointer sends pointer to move/click at a coordinate.
func (instance *Renderer) Pointer(actionType, button string, x, y int) error {
	return instance.SendRequest("pointer", map[string]interface{}{
		"type":   actionType,
		"x":      x,
		"y":      y,
		"button": button,
	}, nil)
}

const (
	KeyCodeBackspace = 16777219 // KeyCodeBackspace is the phantomJS keyboard key code for backspace key.
	KeyCodeEnter     = 16777221 // KeyCodeEnter is the phantomJS keyboard key code for Enter key (works better than Return key!)
)

// SendKey either sends a key string or a key code into the currently focused element on page.
func (instance *Renderer) SendKey(aString string, aCode int64) error {
	if aString != "" {
		instance.SendRequest("type", map[string]interface{}{"key_string": aString}, nil)
	} else if aCode != 0 {
		return instance.SendRequest("type", map[string]interface{}{"key_code": strconv.FormatInt(aCode, 10)}, nil)
	}
	return nil
}

// RemotePageInfo describes the title and URL of the phantomJS page.
type RemotePageInfo struct {
	Title string `json:"title"`
	URL   string `json:"page_url"`
}

// GetPageInfo returns title and URL of the current page.
func (instance *Renderer) GetPageInfo() (info RemotePageInfo, err error) {
	err = instance.SendRequest("info", nil, &info)
	return
}

// LOReset (line-oriented browser) resets recorded element information so that next DOM navigation will find the first element on page.
func (instance *Renderer) LOResetNavigation() error {
	return instance.SendRequest("lo_reset", nil, nil)
}

// ElementInfo tells about an element encountered while navigating around DOM in line-oriented browser.
type ElementInfo struct {
	TagName   string      `json:"tag"`   // TagName is the HTML tag name.
	ID        string      `json:"id"`    // ID is DOM element's ID.
	Name      string      `json:"name"`  // Name is DOM element's name.
	Value     interface{} `json:"value"` // Value is DOM element's value.
	InnerHTML string      `json:"inner"` // InnerHTML is DOM element's inner HTML.
}

// LONext (line-oriented browser) navigates to the next element in DOM. Return information of previous, current, and next element after the action.
func (instance *Renderer) LONextElement() (elements []ElementInfo, err error) {
	elements = make([]ElementInfo, 3)
	err = instance.SendRequest("lo_next", nil, &elements)
	return
}

// LONext (line-oriented browser) navigates to the previous element in DOM.  Return information of previous, current, and next element after the action.
func (instance *Renderer) LOPreviousElement() (elements []ElementInfo, err error) {
	elements = make([]ElementInfo, 3)
	err = instance.SendRequest("lo_prev", nil, &elements)
	return
}

// LONext (line-oriented browser) navigates to the previous element in DOM. Return information of next N elements.
func (instance *Renderer) LONextNElements(n int) (elements []ElementInfo, err error) {
	elements = make([]ElementInfo, 0, n)
	err = instance.SendRequest("lo_next_n", map[string]interface{}{"n": n}, &elements)
	return
}

// LONext (line-oriented browser) sends pointer to click/move to at coordinate of the currently focused element.
func (instance *Renderer) LOPointer(actionType, button string) error {
	return instance.SendRequest("lo_pointer", map[string]interface{}{
		"type":   actionType,
		"button": button,
	}, nil)
}

// LONext (line-oriented browser) sets the value of currently focused element.
func (instance *Renderer) LOSetValue(value string) error {
	return instance.SendRequest("lo_set_val", map[string]interface{}{"value": value}, nil)
}
