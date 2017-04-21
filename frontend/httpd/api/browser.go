package api

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/httpd/api/browser"
	"github.com/HouzuoGuo/laitos/global"
	"io/ioutil"
	"net/http"
	"strconv"
)

const (
	HandleBrowserPage = `<!doctype html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
    <title>%s</title>
    <script type="application/javascript">
        var set_pointer_coord = function (ev, img) {
            var pointer_x = ev.offsetX ? (ev.offsetX) : ev.pageX - img.offsetLeft;
            var pointer_y = ev.offsetY ? (ev.offsetY) : ev.pageY - img.offsetTop;
            document.getElementById('pointer_x').value = pointer_x;
            document.getElementById('pointer_y').value = pointer_y;
        };
    </script>
</head>
<body>
<form action="#" method="post">
    <input type="hidden" name="instance_index" value="%d"/>
    <input type="hidden" name="instance_tag" value="%s"/>
    <table>
        <tr>
            <th>Debug</th>
            <td colspan="5"><textarea rows="5" cols="80">%s</textarea></td>
        </tr>
        <tr>
            <th>View</th>
            <td><input type="submit" name="action" value="Redraw"/></td>
            <td><input type="submit" name="action" value="Kill All"/></td>
            <td>Width: <input type="text" name="view_width" value="%s" size="5"/></td>
            <td>Height: <input type="text" name="view_height" value="%s" size="5"/></td>
            <td>User Agent: <input type="text" name="user_agent" value="%s" size="50"/></td>
        </tr>
        <tr>
            <th>Navigation</th>
            <td><input type="submit" name="action" value="Back"/></td>
            <td><input type="submit" name="action" value="Forward"/></td>
            <td><input type="submit" name="action" value="Reload"/></td>
            <td colspan="2">
                <input type="submit" name="action" value="Go To"/>
                <input type="text" name="page_url" value="%s" size="60"/>
            </td>
        </tr>
        <tr>
            <th>Pointer</th>
            <td><input type="submit" name="action" value="Left Click"/></td>
            <td><input type="submit" name="action" value="Right Click"/></td>
            <td><input type="submit" name="action" value="Move To"/></td>
            <td>X: <input type="text" id="pointer_x" name="pointer_x" value="%s" size="5"/></td>
            <td>Y: <input type="text" id="pointer_y" name="pointer_y" value="%s" size="5"/></td>
        </tr>
        <tr>
            <th>Keyboard</th>
            <td><input type="submit" name="action" value="Backspace"/></td>
            <td><input type="submit" name="action" value="Enter"/></td>
            <td><input type="submit" name="action" value="Type"/></td>
            <td colspan="2">
                <input type="text" name="type_text" value="%s"/>
            </td>
        </tr>
    </table>
    <p><img src="%s?instance_index=%d&instance_tag=%s" onclick="set_pointer_coord(event, this);"/></p>
</form>
</body>
</html>` // Browser page content
	BrowserDebugOutputLen = 1024    // Display this much debug output on page
	BrowserMinWidth       = 1280    // Default browser width in pixels
	BrowserMinHeight      = 3 * 720 // Default browser height in pixels
	BrowserUserAgent      = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.100 Safari/537.36"
)

// Render web page in a server-side javascript-capable browser, and respond with rendered page image.
type HandleBrowser struct {
	ImageEndpoint string            `json:"-"`
	Browsers      browser.Renderers `json:"Browsers"`
}

func (remoteBrowser *HandleBrowser) RenderPage(title string,
	instanceIndex int, instanceTag string,
	debugOut string,
	viewWidth, viewHeight int,
	userAgent, pageUrl string,
	pointerX, pointerY int,
	typeText string) []byte {
	return []byte(fmt.Sprintf(HandleBrowserPage,
		title,
		instanceIndex, instanceTag,
		debugOut,
		strconv.Itoa(viewWidth), strconv.Itoa(viewHeight),
		userAgent, pageUrl,
		strconv.Itoa(pointerX), strconv.Itoa(pointerY),
		typeText,
		remoteBrowser.ImageEndpoint, instanceIndex, instanceTag))
}

func (remoteBrowser *HandleBrowser) ParseSubmission(r *http.Request) (instanceIndex int, instanceTag string, viewWidth, viewHeight int, userAgent, pageUrl string, pointerX, pointerY int, typeText string) {
	instanceIndex, _ = strconv.Atoi(r.FormValue("instance_index"))
	instanceTag = r.FormValue("instance_tag")
	viewWidth, _ = strconv.Atoi(r.FormValue("view_width"))
	viewHeight, _ = strconv.Atoi(r.FormValue("view_height"))
	if viewWidth < BrowserMinWidth {
		viewWidth = BrowserMinWidth
	}
	if viewHeight < BrowserMinHeight {
		viewHeight = BrowserMinHeight
	}
	userAgent = r.FormValue("user_agent")
	pageUrl = r.FormValue("page_url")
	pointerX, _ = strconv.Atoi(r.FormValue("pointer_x"))
	pointerY, _ = strconv.Atoi(r.FormValue("pointer_y"))
	typeText = r.FormValue("type_text")
	return
}
func (remoteBrowser *HandleBrowser) MakeHandler(logger global.Logger, _ *common.CommandProcessor) (http.HandlerFunc, error) {
	if err := remoteBrowser.Browsers.Initialise(); err != nil {
		return nil, err
	}
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		NoCache(w)
		if r.Method == http.MethodGet {
			// Start a new browser instance
			index, instance, err := remoteBrowser.Browsers.Acquire()
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to acquire browser instance: %v", err), http.StatusInternalServerError)
				return
			}
			w.Write(remoteBrowser.RenderPage(
				"Empty Browser",
				index, instance.Tag,
				instance.GetDebugOutput(BrowserDebugOutputLen),
				BrowserMinWidth, BrowserMinHeight, BrowserUserAgent,
				"https://www.google.com",
				0, 0,
				""))
		} else if r.Method == http.MethodPost {
			index, tag, viewWidth, viewHeight, userAgent, pageUrl, pointerX, pointerY, typeText := remoteBrowser.ParseSubmission(r)
			instance := remoteBrowser.Browsers.Retrieve(index, tag)
			if instance == nil {
				// Old instance is no longer there, so start a new browser instance
				index, instance, err := remoteBrowser.Browsers.Acquire()
				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to acquire browser instance: %v", err), http.StatusInternalServerError)
					return
				}
				w.Write(remoteBrowser.RenderPage(
					"Empty Browser",
					index, instance.Tag,
					instance.GetDebugOutput(BrowserDebugOutputLen),
					BrowserMinWidth, BrowserMinHeight, BrowserUserAgent,
					"https://www.google.com",
					0, 0,
					""))
				return
			}
			// Process action on the retrieved browser instance
			switch r.FormValue("action") {
			case "Redraw":
				// There is no javascript action required here
			case "Kill All":
				remoteBrowser.Browsers.KillAll()
			case "Back":
				if err := instance.SendRequest("back", nil, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Forward":
				if err := instance.SendRequest("forward", nil, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Reload":
				if err := instance.SendRequest("reload", nil, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Go To":
				if err := instance.SendRequest("goto", map[string]interface{}{
					"user_agent":  userAgent,
					"page_url":    pageUrl,
					"view_width":  viewWidth,
					"view_height": viewHeight,
				}, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Left Click":
				if err := instance.SendRequest("pointer", map[string]interface{}{
					"type":   "click",
					"x":      pointerX,
					"y":      pointerY,
					"button": "left",
				}, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Right Click":
				if err := instance.SendRequest("pointer", map[string]interface{}{
					"type":   "click",
					"x":      pointerX,
					"y":      pointerY,
					"button": "right",
				}, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Move To":
				if err := instance.SendRequest("pointer", map[string]interface{}{
					"type":   "mosuemove",
					"x":      pointerX,
					"y":      pointerY,
					"button": "left",
				}, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Backspace":
				if err := instance.SendRequest("type", map[string]interface{}{"key_code": "16777219"}, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Enter":
				if err := instance.SendRequest("type", map[string]interface{}{"key_code": "16777221"}, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "Type":
				if err := instance.SendRequest("type", map[string]interface{}{"key_string": typeText}, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			var remotePageInfo struct {
				Title   string `json:"title"`
				PageURL string `json:"page_url"`
			}
			if err := instance.SendRequest("info", nil, &remotePageInfo); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write(remoteBrowser.RenderPage(
				remotePageInfo.Title,
				index, instance.Tag,
				instance.GetDebugOutput(BrowserDebugOutputLen),
				viewWidth, viewHeight,
				userAgent, remotePageInfo.PageURL,
				pointerX, pointerY,
				typeText))
		}
	}
	return fun, nil
}

func (remoteBrowser *HandleBrowser) GetRateLimitFactor() int {
	return 10
}

type HandleBrowserImage struct {
	Browsers *browser.Renderers `json:"-"` // Reference to browser instances constructed in HandleBrowser handler
}

func (remoteBrowserImage *HandleBrowserImage) MakeHandler(logger global.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		NoCache(w)
		index, err := strconv.Atoi(r.FormValue("instance_index"))
		if err != nil {
			http.Error(w, "Bad instance_index", http.StatusBadRequest)
			return
		}
		instance := remoteBrowserImage.Browsers.Retrieve(index, r.FormValue("instance_tag"))
		if instance == nil {
			http.Error(w, "That browser session expired", http.StatusBadRequest)
			return
		}
		if err := instance.RenderPage(); err != nil {
			http.Error(w, "Render error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		pngFile, err := ioutil.ReadFile(instance.RenderImagePath)
		if err != nil {
			http.Error(w, "File IO error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(pngFile)))
		w.Write(pngFile)
	}
	return fun, nil
}

func (_ *HandleBrowserImage) GetRateLimitFactor() int {
	return 10
}
