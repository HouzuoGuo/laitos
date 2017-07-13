try {
    var browser; // the browser page instance after very first URL is visited

    // ============== ACTIONS COMMON TO INTERACTIVE AND LINE-ORIENTED INTERFACE ==========

    // Re-render page screenshot.
    var b_redraw = function () {
        if (!browser) {
            return false;
        }
        browser.render('render.jpg');
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
    var server = require('webserver').create().listen('127.0.0.1:12345', function (req, resp) {
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

    // Reset previous element information, so that the next "next" action will find the first element.
    var b_lo_reset = function () {
        before_info = null;
        exact_info = null;
        after_info = null;
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
        var ret = browser.evaluate(function () {
            return laitos_pjs_find_before_after(laitos_pjs_tag, laitos_pjs_id, laitos_pjs_name, laitos_pjs_inner);
        });
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
        var ret = browser.evaluate(function () {
            return laitos_pjs_find_before_after(laitos_pjs_tag, laitos_pjs_id, laitos_pjs_name, laitos_pjs_inner);
        });

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

        var ret = browser.evaluate(function () {
            return laitos_pjs_find_after(laitos_pjs_tag, laitos_pjs_id, laitos_pjs_name, laitos_pjs_inner, laitos_pjs_next_n);
        });

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

} catch
    (err) {
    var msg = "\nJavascript Program Exception";
    msg += "\nError: " + err.toString();
    for (var p in err) {
        msg += "\n" + p.toUpperCase() + ": " + ex[p];
    }
    console.log(msg);
}