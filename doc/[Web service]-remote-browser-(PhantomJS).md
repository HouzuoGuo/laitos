# Web service: remote browser (PhantomJS)

## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the service renders websites
on laitos server and let you interact with it via mouse and keyboard input. It may be used for:
- Browsing websites internal to laitos host.
- Enable outdated computers (e.g. Windows 95 + Mosaic browser) to enjoy rich experience of modern web.

Be ware that PhantomJS is an old software that may not render some modern web sites. Consider using the remote browser
based on [SlimerJS](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-remote-browser-(SlimerJS)), which
is better in many ways.

## Configuration
Construct the following properties under JSON key `HTTPHandlers`:
1. A string property called `BrowserPhantomJSEndpoint`, value being the URL location that will serve the service. Keep the
   location a secret to yourself and make it difficult to guess.
2. An object called `BrowserPhantomJSEndpointConfig` and its inner object `Browsers` that has the following properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>BasePortNumber</td>
    <td>integer</td>
    <td>
        An arbitrary number above 20000 and below 65535.
        <br/>
        It must not clash with port numbers from other components, such as that used by SlimerJS based remote browser, and
        the "interactive web browser" app.
    </td>
    <td>(This is a mandatory property without a default value)
</tr>
<tr>
    <td>PhantomJSExecPath</td>
    <td>string</td>
    <td>Relative or absolute path to PhantomJS software executable.</td>
    <td>(This is a mandatory property without a default value)</td>  
</tr>
<tr>
    <td>MaxInstances</td>
    <td>integer</td>
    <td>
        Maximum number of websites ("instances") that laitos server will render at the same time. It also determines the
        range of ports (from BasePortNumber to BasePortNumber+MaxInstances) that will be occupied during usage.
        <br/>
        If users open more websites than this number allows, the server will close the oldest websites.
        <br/>
        As rendering consumes a lot of memory, try to keep this number low. 3 is usually a good enough.
    </td>
    <td>5 - good enough for one user</td>
</tr>
<tr>
    <td>MaxLifetimeSec</td>
    <td>integer</td>
    <td>Stop a browser instance after this number of seconds elapse, regardless of whether the instance is in-use.</td>
    <td>1800 - good enough for most case</td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "BrowserPhantomJSEndpoint": "/very-secret-browser-in-browser",
        "BrowserPhantomJSEndpointConfig": {
            "Browsers": {
                "BasePortNumber": 41412,
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
The form is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
In a web browser, navigate to `BrowserPhantomJSEndpoint` of laitos web server.

To browse a website:
- Enter URL to browse in "Go To" text box, then click the button "Go To".
- Wait a second, and click "Redraw" button to view the webpage.

To use mouse:
- On the rendered website screenshot, click at a location of your interest. If your browser cannot understand
  Javascript, you have to manually enter location coordinates into X and Y text boxes.
- Click "LClick", "RClick", or "Move To" to send a mouse command at the interested location.

To use keyboard:
- Send a mouse command such as "LClick" to focus on a text box.
- Enter text you wish to type into the text box next to "Type" button.
- Click "Type" button to send the text to web page.

Additionally, "Backspace" button discards a character, and "Enter" button sends an enter key to web page.

While using the browser, you must regularly click "Redraw" button to view the latest rendered page.

## Tips
- The instance port number from configuration is only for internal localhost use. They do not have to be open on your
  network firewall.
- The web service relies on PhantomJS software that has several software dependencies:
  * bzip2, expat, zlib, fontconfig.
  * Various fonts.

  You may install the software dependencies manually, or reply on [system maintenance](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance)
to automatically install the dependencies.
- There is a latest copy of PhantomJS software in [laitos source tree](https://github.com/HouzuoGuo/laitos/blob/master/extra/linux/phantomjs-2.1.1-x86_64).
