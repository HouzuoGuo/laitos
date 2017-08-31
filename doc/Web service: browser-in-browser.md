# Web service: browser-in-browser

## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server), the service renders websites
on laitos server and let you interact with it via mouse and keyboard input. It may be used for:
- Browsing websites internal to laitos host.
- Enable outdated computers (e.g. Windows 98 + IE 5) to enjoy rich experience of modern web.

## Configuration
Construct the following properties under JSON key `HTTPHandlers`:
1. A string property called `BrowserEndpoint`, value being the URL location that will serve the service. Keep the
   location a secret to yourself and make it difficult to guess.
2. A JSON object `BrowserEndpointConfig` with only one property `Browsers`:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>BasePortNumber</td>
    <td>integer</td>
    <td>
        An arbitrary number above 20000.
        <br/>
        It must not clash with BasePortNumber from other components' configuration.
    </td>
</tr>
<tr>
    <td>MaxInstances</td>
    <td>integer</td>
    <td>
        Maximum number of websites ("instances") that laitos server will render at the same time.
        <br/>
        If users open more websites than this number allows, the server will close the oldest websites.
        <br/>
        As rendering consumes a lot of memory, try to keep this number low. 3 is usually a good enough.
    </td>
</tr>
<tr>
    <td>MaxLifetimeSec</td>
    <td>integer</td>
    <td>Stop a browser instance after this number of seconds elapse, regardless of whether the instance is in-use.</td>
</tr>
<tr>
    <td>PhantomJSExecPath</td>
    <td>string</td>
    <td>
        Absolute or relative path to PhantomJS executable. You may download it from PhantomJS website, or acquire a copy
        from <a href="https://github.com/HouzuoGuo/laitos/tree/master/addon">laitos source tree</a>.
    </td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "BrowserEndpoint": "/very-secret-browser-in-browser",
        "BrowserEndpointConfig": {
            "Browsers": {
                "BasePortNumber": 1412,
                "MaxInstances": 5,
                "MaxLifetimeSec": 1800,
                "PhantomJSExecPath": "./phantomjs-2.1.1-linux-x86_64"
            }
        }

        ...
    },

    ...
}
</pre>

## Run
The form is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server#run).

## Usage
In a web browser, navigate to `BrowserEndpoint` of laitos web server.

To browse a website:
- Enter URL to browse in "Go To" text box, then click the button "Go To".
- Wait a second, and click "Redraw" button to view the webpage.

To use mouse:
- On the rendered website screenshot, click at a location of your interest. If your browser cannot understand
  Javascript, you have to manually enter location coordinates into X and Y text boxes.
- Click "Left Click", "Right Click", or "Move To" to send a mouse command at the interested location.

To use keyboard:
- Send a mouse command such as "Left Click" to focus on a text box.
- Enter text you wish to type into the text box next to "Type" button.
- Click "Type" button to send the text to web page.

Additionally, "Backspace" button discards a character, and "Enter" button sends an enter key to web page.

While using the browser, you must regularly click "Redraw" button to view the latest rendered page. 

## Tips
Make sure to choose a very secure URL for the endpoint, it is the only way to secure this web service!

The web service relies on PhantomJS software that has several software dependencies:
- bzip2, expat, zlib, fontconfig.
- Various fonts.

You may install the software dependencies manually, or reply on [system maintenance](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-system-maintenance)
to automatically install the dependencies.

If you wish to use the web service on a legacy browser such as IE 5.5, then remember to start plain HTTP daemon
(i.e. `insecurehttpd`), because legacy browsers do not support modern TLS (HTTPS). 