var webPage = require('webpage');

var page = webPage.create();

page.onConsoleMessage = function (msg, lineNum, sourceId) {
    console.log(msg);
};

page.settings.userAgent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36';

page.open('https://www.google.com', function (status) {
    try {
        console.log(status);

        page.evaluate(function () {
            var walk = function (elem, walk_fun) {
                for (var child = elem.childNodes, t = 0; t < child.length; t++) {
                    if (!walk(child[t], walk_fun)) {
                        return false;
                    }
                }
                return walk_fun(elem);
            };

            var find_before_after = function (tag, id, name, inner) {
                var before, exact, after, stop_next = false;
                walk(document.documentElement, function (elem) {
                    var height = elem.offsetHeight,
                        width = elem.offsetWidth,
                        elem_inner = elem.innerHTML;
                    if (height > 3 && width > 3 && elem_inner.length < 1000) {
                        console.log('ID-' + elem.id + ' TAG-' + elem.tagName + ' NAME-' + elem.name + ' VALUE-' + elem.value + ' INNER-' + elem.innerHTML);
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
                return [before, exact, after];
            };

            var find_after = function (tag, id, name, inner, num) {
                var ret = [], matched = false;
                walk(document.documentElement, function (elem) {
                    var height = elem.offsetHeight,
                        width = elem.offsetWidth,
                        elem_inner = elem.innerHTML;
                    if (height > 3 && width > 3 && elem_inner.length < 1000) {
                        if (elem.tagName === tag && elem.id === id && elem.name === name && elem_inner === inner) {
                            matched = true;
                            return true;
                        }
                        if (matched) {
                            if (ret.length < num) {
                                ret.push(elem);
                            } else {
                                return false;
                            }
                        }
                    }
                    return true;
                });
                return ret;
            };

            var ret = find_before_after('INPUT', 'gs_taif0', '', '');
            console.log('before: ' + ret[0].id + ' exact: ' + ret[1].id + ' after: ' + ret[2].id);

            var eight = find_after('INPUT', 'gs_taif0', '', '', 8);
            for (var i = 0; i < 8; i++) {
                var elem = eight[i];
                console.log('ID-' + elem.id + ' TAG-' + elem.tagName + ' NAME-' + elem.name + ' VALUE-' + elem.value + ' INNER-' + elem.innerHTML);
            }
        });

        setTimeout(function () {
            page.render('render.jpg');
        }, 2000);

    } catch (ex) {
        var fullMessage = "\nJAVASCRIPT EXCEPTION";
        fullMessage += "\nMESSAGE: " + ex.toString();
        for (var p in ex) {
            fullMessage += "\n" + p.toUpperCase() + ": " + ex[p];
        }
        console.log(fullMessage);
    }
});