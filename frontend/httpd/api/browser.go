package api

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/httpclient"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
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
    <input type="hidden" name="browser_id" value=""/>
    <table>
        <tr>
            <th>Debug</th>
            <td colspan="5"><textarea rows="5" cols="80">%s</textarea></td>
        </tr>
        <tr>
            <th>View</th>
            <td><input type="submit" name="action" value="Redraw"/></td>
            <td>Width: <input type="text" name="view_width" value="%s" size="5"/></td>
            <td>Height: <input type="text" name="view_height" value="%s" size="5"/></td>
            <td>User Agent: <input type="text" name="user_agent" value="%s" size="50"/></td>
        </tr>
        <tr>
            <th>Navigation</th>
            <td><input type="submit" name="action" value="Back"/></td>
            <td><input type="submit" name="action" value="Forward"/></td>
            <td><input type="submit" name="action" value="Reload"/></td>
            <td>
                <input type="submit" name="action" value="Go To"/>
                <input type="text" name="page_url" value="%s" size="60"/>
            </td>
        </tr>
        <tr>
            <th>Pointer</th>
            <td><input type="submit" name="action" value="Left Click"/></td>
            <td><input type="submit" name="action" value="Right Click"/></td>
            <td><input type="submit" name="action" value="Move To"/></td>
            <td>
                X: <input type="text" id="pointer_x" name="pointer_x" value="%s" size="5"/>&nbsp;
                Y: <input type="text" id="pointer_y" name="pointer_y" value="%s" size="5"/>
            </td>
        </tr>
        <tr>
            <th>Keyboard</th>
            <td><input type="submit" name="action" value="Backspace"/></td>
            <td><input type="submit" name="action" value="Enter"/></td>
            <td colspan="2">
                <input type="submit" name="action" value="Type"/>
                <input type="text" name="type_text" value="%s"/>
            </td>
        </tr>
    </table>
    <p><img src="%s" onclick="set_pointer_coord(event, this);"/></p>
</form>
</body>
</html>` // Browser page content
	BrowserMinWidth  = 1280
	BrowserMinHeight = 3 * 720
	BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.100 Safari/537.36"
)

var browserStarted bool

type HandleBrowser struct {
	ImageEndpoint string `json:"-"`
}

func (browser *HandleBrowser) renderPage(title, debugOut string,
	viewWidth, viewHeight int,
	userAgent, pageUrl string,
	pointerX, pointerY int,
	typeText string) []byte {
	return []byte(fmt.Sprintf(HandleBrowserPage, title, debugOut,
		strconv.Itoa(viewWidth), strconv.Itoa(viewHeight),
		userAgent, pageUrl,
		strconv.Itoa(pointerX), strconv.Itoa(pointerY),
		typeText,
		browser.ImageEndpoint))
}

func (browser *HandleBrowser) parseSubmission(r *http.Request) (viewWidth, viewHeight int, userAgent, pageUrl string, pointerX, pointerY int, typeText string) {
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
func (browser *HandleBrowser) MakeHandler(logger global.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		if !browserStarted {
			os.Remove("/home/howard/page.png")
			go func() {
				cmd := exec.Command("/home/howard/gopath/src/github.com/HouzuoGuo/laitos/phantomjs-2.1.1-linux-x86_64/bin/phantomjs", "/home/howard/gopath/src/github.com/HouzuoGuo/laitos/phantomjs-2.1.1-linux-x86_64/bin/main.js")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Start(); err != nil {
					logger.Fatalf("browser", "browser", err, "failed to start browser")
				}
			}()
			// Give javascript a second to start
			time.Sleep(1 * time.Second)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		NoCache(w)
		if r.Method == http.MethodGet {
			w.Write(browser.renderPage("Empty Title", "Empty Output",
				BrowserMinWidth, BrowserMinHeight, BrowserUserAgent,
				"https://www.google.com",
				0, 0,
				"sample text"))
		} else if r.Method == http.MethodPost {
			viewWidth, viewHeight, userAgent, pageUrl, pointerX, pointerY, typeText := browser.parseSubmission(r)
			switch r.FormValue("action") {
			case "Redraw":
				// There is no javascript action required here
			case "Back":
				resp, err := httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, "http://127.0.0.1:12345/back")
				logger.Printf("back", "", err, "resp body is: %s", string(resp.Body))
			case "Forward":
				resp, err := httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, "http://127.0.0.1:12345/forward")
				logger.Printf("forward", "", err, "resp body is: %s", string(resp.Body))
			case "Reload":
				resp, err := httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, "http://127.0.0.1:12345/reload")
				logger.Printf("reload", "", err, "resp body is: %s", string(resp.Body))
			case "Go To":
				resp, err := httpclient.DoHTTP(httpclient.Request{
					Method: http.MethodPost,
					Body: strings.NewReader(url.Values{
						"user_agent":  []string{userAgent},
						"page_url":    []string{pageUrl},
						"view_width":  []string{strconv.Itoa(viewWidth)},
						"view_height": []string{strconv.Itoa(viewHeight)},
					}.Encode()),
				}, "http://127.0.0.1:12345/goto")
				logger.Printf("goto", "", err, "resp body is: %s", string(resp.Body))
			case "Left Click":
				resp, err := httpclient.DoHTTP(httpclient.Request{
					Method: http.MethodPost, Body: strings.NewReader(url.Values{
						"type":   []string{"click"},
						"x":      []string{strconv.Itoa(pointerX)},
						"y":      []string{strconv.Itoa(pointerY)},
						"button": []string{"left"},
					}.Encode())}, "http://127.0.0.1:12345/pointer")
				logger.Printf("left click", "", err, "resp body is: %s", string(resp.Body))
			case "Right Click":
				resp, err := httpclient.DoHTTP(httpclient.Request{
					Method: http.MethodPost, Body: strings.NewReader(url.Values{
						"type":   []string{"click"},
						"x":      []string{strconv.Itoa(pointerX)},
						"y":      []string{strconv.Itoa(pointerY)},
						"button": []string{"right"},
					}.Encode())}, "http://127.0.0.1:12345/pointer")
				logger.Printf("right click", "", err, "resp body is: %s", string(resp.Body))
			case "Move To":
				resp, err := httpclient.DoHTTP(httpclient.Request{
					Method: http.MethodPost, Body: strings.NewReader(url.Values{
						"type":   []string{"mousemove"},
						"x":      []string{strconv.Itoa(pointerX)},
						"y":      []string{strconv.Itoa(pointerY)},
						"button": []string{"left"},
					}.Encode())}, "http://127.0.0.1:12345/pointer")
				logger.Printf("right click", "", err, "resp body is: %s", string(resp.Body))
			case "Backspace":
			case "Enter":
			case "Type":
			}
			w.Write(browser.renderPage("Empty Title", "Empty Output",
				viewWidth, viewHeight,
				userAgent, pageUrl,
				pointerX, pointerY,
				typeText))
		}
	}
	return fun, nil
}

func (_ *HandleBrowser) GetRateLimitFactor() int {
	return 10
}

type HandleBrowserImage struct {
}

func (_ *HandleBrowserImage) MakeHandler(logger global.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		NoCache(w)
		time.Sleep(100 * time.Millisecond)
		resp, err := httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, "http://127.0.0.1:12345/redraw")
		logger.Printf("capture", "capture", err, "resp body is: %s", string(resp.Body))
		if err == nil {
			time.Sleep(100 * time.Millisecond)
			pngFile, err := ioutil.ReadFile("/home/howard/page.png")
			w.Header().Set("Content-Length", strconv.Itoa(len(pngFile)))
			if err != nil {
				logger.Printf("capture", "capture", err, "failed to open png file")
			}
			w.Write(pngFile)
		}
	}
	return fun, nil
}

func (_ *HandleBrowserImage) GetRateLimitFactor() int {
	return 10
}
