package headless_browser

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
)

const (
	BrowserServerCodeTemplate = `var browser;

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
    browser.open(param.page_url, function (result) {
        console.log('b_open: ' + result);
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

var b_close = function () {
    if (!browser) {
        return false;
    }
    browser.close();
    browser = null;
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
    } else if (req.url === '/close') {
        ret = b_close();
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

// An instance of headless browser server that acts on commands received via HTTP.
type BrowserServer struct {
	PhantomJSExecPath string       `json:"-"` // Absolute or relative path to PhantomJS executable
	DebugOutput       bytes.Buffer `json:"-"` // Store standard output and error from PhantomJS executable
	RenderImagePath   string       `json:"-"` // Place to store rendered web page image
	Port              int          `json:"-"` // Port number for headless server to listen for commands on
	PID               int          `json:"-"` // PID of headless server
}

// Produce javascript code for browser server and then launch its process in background.
func (browser *BrowserServer) Start() error {
	_, err := ioutil.TempFile("", "laitos-browser-server")
	if err != nil {
		return fmt.Errorf("BrowserServer.Start: failed to create temporary file for PhantomJS code - %v", err)
	}
}
