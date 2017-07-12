try {
    var browser; // browser is the browser page instance

    // ============== ACTIONS COMMON TO INTERACTIVE AND LINE-ORIENTED INTERFACE ==========
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
            browser.onConsoleMessage = function (msg, line_num, src_id) {
                console.log("PAGE CONSOLE: " + msg);
            };
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

    var server = require('webserver').create().listen('127.0.0.1:12345', function (req, resp) {
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

    // ===================== ONLY FOR LINE-ORIENTED INTERFACE =================
    var last_info = null, prev_info = null;

    var quote_str = function (str) {
        return JSON.stringify(str);
    };

    var elem_info_to_stmt = function (elem_info) {
        return "function(){" +
            "window.laitos_pjs_tag = " + quote_str(elem_info === null ? '' : elem_info['tag']) + ";" +
            "window.laitos_pjs_id  = " + quote_str(elem_info === null ? '' : elem_info['id']) + ";" +
            "window.laitos_pjs_name = " + quote_str(elem_info === null ? '' : elem_info['name']) + ";" +
            "window.laitos_pjs_inner = " + quote_str(elem_info === null ? '' : elem_info['inner']) + ";" +
            "}";
    };

    var eval_find_before_after = function () {
        var elem_to_obj = function (elem) {
            return {
                "tag": elem.tagName,
                "id": elem.id,
                "name": elem.name,
                "value": elem.value,
                "inner": elem.innerHTML
            };
        };

        var walk = function (elem, walk_fun) {
            for (var child = elem.childNodes, t = 0; t < child.length; t++) {
                if (!walk(child[t], walk_fun)) {
                    return false;
                }
            }
            return walk_fun(elem);
        };

        var find_before_after = function (tag, id, name, inner) {
            var before = null, exact = null, after = null, stop_next = false;
            walk(document.documentElement, function (elem) {
                var height = elem.offsetHeight,
                    width = elem.offsetWidth,
                    elem_inner = elem.innerHTML;
                if (height > 3 && width > 3 && elem_inner.length < 1000) {
                    if (stop_next) {
                        after = elem;
                        return false;
                    }
                    if (elem.tagName === tag && elem.id === id && elem.name === name && elem_inner === inner) {
                        exact = elem;
                        stop_next = true;
                    } else {
                        before = elem;
                    }
                }
                return true;
            });
            return [
                before === null ? null : elem_to_obj(before),
                exact === null ? null : elem_to_obj(exact),
                after === null ? null : elem_to_obj(after)
            ];
        };

        return find_before_after(laitos_pjs_tag, laitos_pjs_id, laitos_pjs_name, laitos_pjs_inner);
    };

    var b_lo_reset = function () {
        prev_info = null;
        last_info = null;
    };

    var b_lo_next = function () {
        if (!browser) {
            return false;
        }

        browser.evaluateJavaScript(elem_info_to_stmt(last_info));
        var ret = browser.evaluate(eval_find_before_after());

        prev_info = ret[0];
        last_info = ret[1];
        return ret;
    };

    var b_lo_prev = function () {
        if (!browser) {
            return false;
        }

        browser.evaluateJavaScript(elem_info_to_stmt(prev_info));
        var ret = browser.evaluate(eval_find_before_after());

        prev_info = ret[0];
        last_info = ret[1];
        return ret;
    };

    var b_lo_next_n = function (n) {

    };

} catch (err) {
    var msg = "\nJavascript Program Exception";
    msg += "\nError: " + err.toString();
    for (var p in err) {
        msg += "\n" + p.toUpperCase() + ": " + ex[p];
    }
    console.log(msg);
}