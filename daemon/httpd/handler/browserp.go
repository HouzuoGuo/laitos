package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/browser/phantomjs"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	HandleBrowserPage = `<html>
<head>
    <title>%s</title>
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
    <input type="hidden" name="instance_index" value="%d"/>
    <input type="hidden" name="instance_tag" value="%s"/>
    <p>%s</p>
    <table>
        <tr>
            <th>Debug</th>
            <td colspan="5"><textarea rows="2" cols="60">%s</textarea></td>
        </tr>
        <tr>
            <th>Browser</th>
            <td>Width<input type="text" name="width" value="%s" size="2"/></td>
            <td>Height<input type="text" name="height" value="%s" size="2"/></td>
            <td>User Agent: <input type="text" name="user_agent" value="%s" size="6"/></td>
            <td><input type="submit" name="action" value="Reload"/></td>
            <td><input type="submit" name="action" value="Kill All"/></td>
		</tr>
		<tr>
            <th>Draw</th>
            <td><input type="submit" name="action" value="Redraw"/></td>
            <td>Top<input type="text" name="top" value="%s" size="3"/></td>
            <td>Left<input type="text" name="left" value="%s" size="3"/></td>
			<td>Width<input type="text" name="draw_width" value="%s" size="3"/></td>
			<td>Height<input type="text" name="draw_height" value="%s" size="3"/></td>
        </tr>
        <tr>
            <th>Navigation</th>
            <td><input type="submit" name="action" value="Back"/></td>
            <td><input type="submit" name="action" value="Forward"/></td>
            <td colspan="3">
                <input type="submit" name="action" value="Go To"/>
                <input type="text" name="page_url" value="%s" size="30"/>
            </td>
        </tr>
        <tr>
            <th>Mouse</th>
            <td><input type="submit" name="action" value="LClick"/></td>
            <td><input type="submit" name="action" value="RClick"/></td>
            <td><input type="submit" name="action" value="Move To"/></td>
            <td>X<input type="text" id="pointer_x" name="pointer_x" value="%s" size="2"/></td>
            <td>Y<input type="text" id="pointer_y" name="pointer_y" value="%s" size="2"/></td>
		</tr>
		<tr>
            <th>Keyboard</th>
            <td><input type="submit" name="action" value="Backspace"/></td>
            <td><input type="submit" name="action" value="Enter"/></td>
            <td><input type="submit" name="action" value="Type"/></td>
            <td colspan="2">
                <input type="text" name="type_text" value="%s" size="20"/>
            </td>
        </tr>
    </table>
    <p><img id="render" src="%s?instance_index=%d&instance_tag=%s&rand=%d" alt="rendered page" onclick="set_pointer_coord(event);"/></p>
</form>
</body>
</html>` // Browser page content
)

// Render web page in a server-side javascript-capable browser, and respond with rendered page image.
type HandleBrowserPhantomJS struct {
	ImageEndpoint              string              `json:"-"`
	Browsers                   phantomjs.Instances `json:"Browsers"`
	stripURLPrefixFromResponse string
}

func (remoteBrowser *HandleBrowserPhantomJS) Initialise(_ lalog.Logger, _ *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	remoteBrowser.stripURLPrefixFromResponse = stripURLPrefixFromResponse
	return remoteBrowser.Browsers.Initialise()
}

// RenderControlPage returns string text of the web page that offers remote browser controls.
func RenderControlPage(stripURLPrefixFromResponse, title, requestURI string,
	instanceIndex int, instanceTag string,
	lastErr error, debugOut string,
	viewWidth, viewHeight int, userAgent string,
	drawTop, drawLeft, drawWidth, drawHeight int,
	pageUrl string,
	pointerX, pointerY int,
	typeText, browserImageEndpoint string) []byte {
	var errStr string
	if lastErr == nil {
		errStr = ""
	} else {
		errStr = lastErr.Error()
	}
	return []byte(fmt.Sprintf(HandleBrowserPage,
		title, strings.TrimPrefix(requestURI, stripURLPrefixFromResponse),
		instanceIndex, instanceTag,
		errStr, debugOut,
		strconv.Itoa(viewWidth), strconv.Itoa(viewHeight), userAgent,
		strconv.Itoa(drawTop), strconv.Itoa(drawLeft), strconv.Itoa(drawWidth), strconv.Itoa(drawHeight),
		pageUrl,
		strconv.Itoa(pointerX), strconv.Itoa(pointerY),
		typeText,
		strings.TrimPrefix(browserImageEndpoint, stripURLPrefixFromResponse), instanceIndex, instanceTag, time.Now().UnixNano()))
}

func (remoteBrowser *HandleBrowserPhantomJS) parseSubmission(r *http.Request) (instanceIndex int, instanceTag string,
	viewWidth, viewHeight int, userAgent string,
	drawTop, drawLeft, drawWidth, drawHeight int,
	pageUrl string, pointerX, pointerY int, typeText string,
) {

	instanceIndex, _ = strconv.Atoi(r.FormValue("instance_index"))
	instanceTag = r.FormValue("instance_tag")

	viewWidth, _ = strconv.Atoi(r.FormValue("width"))
	viewHeight, _ = strconv.Atoi(r.FormValue("height"))
	userAgent = r.FormValue("user_agent")

	drawTop, _ = strconv.Atoi(r.FormValue("top"))
	drawLeft, _ = strconv.Atoi(r.FormValue("left"))
	drawWidth, _ = strconv.Atoi(r.FormValue("draw_width"))
	drawHeight, _ = strconv.Atoi(r.FormValue("draw_height"))

	pageUrl = r.FormValue("page_url")

	pointerX, _ = strconv.Atoi(r.FormValue("pointer_x"))
	pointerY, _ = strconv.Atoi(r.FormValue("pointer_y"))
	typeText = r.FormValue("type_text")
	return
}

func (remoteBrowser *HandleBrowserPhantomJS) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	NoCache(w)
	if r.Method == http.MethodGet {
		// Start a new browser instance
		index, instance, err := remoteBrowser.Browsers.Acquire()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to acquire browser instance: %v", err), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(RenderControlPage(remoteBrowser.stripURLPrefixFromResponse,
			"Empty Browser", r.RequestURI,
			index, instance.Tag,
			nil, instance.GetDebugOutput(),
			800, 800, phantomjs.GoodUserAgent,
			0, 0, 800, 800,
			"https://www.google.com",
			0, 0,
			"", remoteBrowser.ImageEndpoint))
	} else if r.Method == http.MethodPost {
		index, tag, viewWidth, viewHeight, userAgent, drawTop, drawLeft, drawWidth, drawHeight, pageUrl, pointerX, pointerY, typeText := remoteBrowser.parseSubmission(r)
		instance := remoteBrowser.Browsers.Retrieve(index, tag)
		if instance == nil {
			// Old instance is no longer there, so start a new browser instance
			index, instance, err := remoteBrowser.Browsers.Acquire()
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to acquire browser instance: %v", err), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(RenderControlPage(remoteBrowser.stripURLPrefixFromResponse,
				"Empty Browser", r.RequestURI,
				index, instance.Tag,
				nil, instance.GetDebugOutput(),
				800, 800, phantomjs.GoodUserAgent,
				drawTop, drawLeft, drawWidth, drawHeight,
				"https://www.google.com",
				0, 0,
				"", remoteBrowser.ImageEndpoint))
			return
		}
		var actionErr error
		switch r.FormValue("action") {
		case "Redraw":
			// Set draw region, and the image responded by its own dedicated endpoint will pick it up.
			actionErr = instance.SetRenderArea(drawTop, drawLeft, drawWidth, drawHeight)
		case "Kill All":
			remoteBrowser.Browsers.KillAll()
			actionErr = errors.New("All browser sessions are gone. Please nagivate back to this browser page by re-entering the URL, do not refresh the page.")
		case "Back":
			actionErr = instance.GoBack()
		case "Forward":
			actionErr = instance.GoForward()
		case "Reload":
			actionErr = instance.Reload()
		case "Go To":
			actionErr = instance.GoTo(userAgent, pageUrl, viewWidth, viewHeight)
		case "LClick":
			actionErr = instance.Pointer(phantomjs.PointerTypeClick, phantomjs.PointerButtonLeft, pointerX, pointerY)
		case "RClick":
			actionErr = instance.Pointer(phantomjs.PointerTypeClick, phantomjs.PointerButtonRight, pointerX, pointerY)
		case "Move To":
			actionErr = instance.Pointer(phantomjs.PointerTypeMove, phantomjs.PointerButtonLeft, pointerX, pointerY)
		case "Backspace":
			actionErr = instance.SendKey("", phantomjs.KeyCodeBackspace)
		case "Enter":
			actionErr = instance.SendKey("", phantomjs.KeyCodeEnter)
		case "Type":
			actionErr = instance.SendKey(typeText, 0)
		}
		// Display action error, or page info error if there is any.
		pageInfo, pageInfoErr := instance.GetPageInfo()
		if actionErr == nil {
			actionErr = pageInfoErr
		}
		_, _ = w.Write(RenderControlPage(remoteBrowser.stripURLPrefixFromResponse,
			pageInfo.Title, r.RequestURI,
			index, instance.Tag,
			actionErr, instance.GetDebugOutput(),
			viewWidth, viewHeight, userAgent,
			drawTop, drawLeft, drawWidth, drawHeight,
			pageInfo.URL,
			pointerX, pointerY,
			typeText, remoteBrowser.ImageEndpoint))
	}
}

func (_ *HandleBrowserPhantomJS) GetRateLimitFactor() int {
	return 2
}

func (_ *HandleBrowserPhantomJS) SelfTest() error {
	return nil
}

type HandleBrowserPhantomJSImage struct {
	Browsers *phantomjs.Instances `json:"-"` // Reference to browser instances constructed in HandleBrowser handler
}

func (_ *HandleBrowserPhantomJSImage) Initialise(lalog.Logger, *toolbox.CommandProcessor, string) error {
	return nil
}

func (remoteBrowserImage *HandleBrowserPhantomJSImage) Handle(w http.ResponseWriter, r *http.Request) {
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
	pngFile, err := os.ReadFile(instance.RenderImagePath)
	if err != nil {
		http.Error(w, "File IO error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(pngFile)))
	_, _ = w.Write(pngFile)
}

func (_ *HandleBrowserPhantomJSImage) GetRateLimitFactor() int {
	return 3
}

func (_ *HandleBrowserPhantomJSImage) SelfTest() error {
	return nil
}
