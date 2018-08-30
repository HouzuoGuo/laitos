# Web service: remote browser (SlimerJS)

## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server), the service renders websites
on laitos server and let you interact with it via mouse and keyboard input. It may be used for:
- Browsing websites internal to laitos host.
- Enable outdated computers (e.g. Windows 95 + Mosaic browser) to enjoy rich experience of modern web.

In contrast to PhantomJS based web browser, SlimerJS based web browser is more capable of rendering very modern
websites, even Google Maps and YouTube. However, SlimerJS based web browsers rely on Docker container runtime or
supplement applications for Windows, which may not be available in your server hosting environment (e.g. Windows
Subsystem For Linux, AWS FarGate).

## Configuration
Construct the following properties under JSON key `HTTPHandlers`:
1. A string property called `BrowserSlimerJSEndpoint`, value being the URL location that will serve the service. Keep the
   location a secret to yourself and make it difficult to guess.
2. An object called `BrowserSlimerJSEndpointConfig` and its inner object `Browsers` that has the following properties:
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
        It must not clash with port numbers from other components, such as the PhantomJS based remote browser, and the
        toolbox "interactive web browser" feature.
    </td>
    <td>(This is a mandatory property without a default value)
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

        "BrowserSlimerJSEndpoint": "/very-secret-browser-in-browser",
        "BrowserSlimerJSEndpointConfig": {
            "Browsers": {
                "BasePortNumber": 28418,
                "MaxInstances": 5,
                "MaxLifetimeSec": 1800
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
In a web browser, navigate to `BrowserSlimerJSEndpoint` of laitos web server.

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
- If laitos host is running Linux, then it will need Docker container runtime and tools to launch SlimerJS. You may
  install Docker daemon and client manually, or reply on [system maintenance](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-system-maintenance)
  to automatically make preparations for Docker.
- If laitos host is Windows, then it will need [supplement programs](https://github.com/HouzuoGuo/laitos-windows-supplements)
  instead of Docker daemon. Download and place the supplements into `laitos-windows-supplements` directory underneath
  C, D, E, or F drive.
- SELinux will be disabled on the host operating system for SlimerJS to function properly.
- You may find out more about the SlimerJS container image over [here](https://hub.docker.com/r/hzgl/slimerjs).