package handler

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/browserp"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
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
    <script type="text/javascript">
        function set_pointer_coord(ev) {
            var pointer_x = ev.offsetX ? (ev.offsetX) : ev.pageX - document.getElementById('render').offsetLeft;
            var pointer_y = ev.offsetY ? (ev.offsetY) : ev.pageY - document.getElementById('render').offsetTop;
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
    <p><img id="render" src="%s?instance_index=%d&instance_tag=%s" alt="rendered page" onclick="set_pointer_coord(event);"/></p>
</form>
</body>
</html>` // Browser page content
)

// Render web page in a server-side javascript-capable browser, and respond with rendered page image.
type HandleBrowser struct {
	ImageEndpoint string             `json:"-"`
	Browsers      browserp.Instances `json:"Browsers"`
}

func (remoteBrowser *HandleBrowser) Initialise(misc.Logger, *common.CommandProcessor) error {
	return remoteBrowser.Browsers.Initialise()
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

func (remoteBrowser *HandleBrowser) parseSubmission(r *http.Request) (instanceIndex int, instanceTag string, viewWidth, viewHeight int, userAgent, pageUrl string, pointerX, pointerY int, typeText string) {
	instanceIndex, _ = strconv.Atoi(r.FormValue("instance_index"))
	instanceTag = r.FormValue("instance_tag")
	viewWidth, _ = strconv.Atoi(r.FormValue("view_width"))
	viewHeight, _ = strconv.Atoi(r.FormValue("view_height"))
	userAgent = r.FormValue("user_agent")
	pageUrl = r.FormValue("page_url")
	pointerX, _ = strconv.Atoi(r.FormValue("pointer_x"))
	pointerY, _ = strconv.Atoi(r.FormValue("pointer_y"))
	typeText = r.FormValue("type_text")
	return
}

func (remoteBrowser *HandleBrowser) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	NoCache(w)
	if !WarnIfNoHTTPS(r, w) {
		return
	}
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
			instance.GetDebugOutput(),
			800, 800, browserp.GoodUserAgent,
			"https://www.google.com",
			0, 0,
			""))
	} else if r.Method == http.MethodPost {
		index, tag, viewWidth, viewHeight, userAgent, pageUrl, pointerX, pointerY, typeText := remoteBrowser.parseSubmission(r)
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
				instance.GetDebugOutput(),
				800, 800, browserp.GoodUserAgent,
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
			if err := instance.GoBack(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Forward":
			if err := instance.GoForward(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Reload":
			if err := instance.Reload(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Go To":
			if err := instance.GoTo(userAgent, pageUrl, viewWidth, viewHeight); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Left Click":
			if err := instance.Pointer(browserp.PointerTypeClick, browserp.PointerButtonLeft, pointerX, pointerY); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Right Click":
			if err := instance.Pointer(browserp.PointerTypeClick, browserp.PointerButtonRight, pointerX, pointerY); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Move To":
			if err := instance.Pointer(browserp.PointerTypeMove, browserp.PointerButtonLeft, pointerX, pointerY); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Backspace":
			if err := instance.SendKey("", browserp.KeyCodeBackspace); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Enter":
			if err := instance.SendKey("", browserp.KeyCodeEnter); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "Type":
			if err := instance.SendKey(typeText, 0); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		pageInfo, err := instance.GetPageInfo()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(remoteBrowser.RenderPage(
			pageInfo.Title,
			index, instance.Tag,
			instance.GetDebugOutput(),
			viewWidth, viewHeight,
			userAgent, pageInfo.URL,
			pointerX, pointerY,
			typeText))
	}
}

func (_ *HandleBrowser) GetRateLimitFactor() int {
	return 2
}

func (_ *HandleBrowser) SelfTest() error {
	return nil
}

type HandleBrowserImage struct {
	Browsers *browserp.Instances `json:"-"` // Reference to browser instances constructed in HandleBrowser handler
}

func (_ *HandleBrowserImage) Initialise(misc.Logger, *common.CommandProcessor) error {
	return nil
}

func (remoteBrowserImage *HandleBrowserImage) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	/*
		There is no need to call WarnIfNoHTTPS function in this API, because this API is not reachable unless
		HandleBrowser has already warned the user.
	*/
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
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(pngFile)))
	w.Write(pngFile)
}

func (_ *HandleBrowserImage) GetRateLimitFactor() int {
	return 2
}

func (_ *HandleBrowserImage) SelfTest() error {
	return nil
}
