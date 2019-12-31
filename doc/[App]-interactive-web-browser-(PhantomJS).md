## Introduction
Via any of enabled laitos daemons, you may browse the Internet via text-based commands.

The website to browse is rendered with full CSS and Javascript support on laitos server. Interaction with the website is
carried out entirely via plain text commands. The command response will offer clue (in plain text) as to how the web
site looks while navigating around. Only one website can be browsed at a time.

Be ware that PhantomJS is an old software that may not render some modern web sites. Consider using the remote browser
based on [SlimerJS](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-interactive-web-browser-(SlimerJS)), which
is better in many ways.

## Configuration
Under JSON object `Features`, construct a JSON object called `BrowserPhantomJS` and its inner object `Browsers` that has
the following properties:
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
        It must not clash with port numbers from other components, such as the SlimerJS based interactive web browser,
        and the remote browser web services.
    </td>
    <td>(This is a mandatory property without a default value)
</tr>
<tr>
    <td>PhantomJSExecPath</td>
    <td>string</td>
    <td>Stop a browser instance after this number of seconds elapse, regardless of whether the instance is in-use.</td>
    <td>(This is a mandatory property without a default value)</td>  
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

    "Features": {
        ...

        "Browser": {
            "Browsers": {
                "BasePortNumber": 51202,
                "MaxLifetimeSec": 1800,
                "PhantomJSExecPath": "./phantomjs-2.1.1-linux-x86_64"
            }
        },
        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to invoke the app.

To visit a website:
- Go to a new URL via `.bg example.com`
- Get current page title and URL via `.bi`
- Reload page via `.br`
- Navigate forward and backward via `.bf` and `.bb`
- Stop browser via `.bk`

To navigate within a page:
- Visit the next element via `.bn`, the response will describe the previous, current, and next element.
- Visit the next N elements via `.bnn #NUMBER`, the response will describe all `#NUMBER` elements along the way.
- Visit the previous element via `.bp`, the response will describe the previous, current, and next element.
- Start over from beginning via `.b0`.

To interact with mouse cursor on the page:
- Make a left click on current element via `.bptr click left`
- Make a right click on current element via `.bptr click right`
- Move mouse to current element via `.bptr move left`

To interact with keyboard on the page:
- Press the Enter key via `.benter`
- Press the Backspace key via `.bbacksp`
- Enter arbitrary text into current element via `.be this is text`
- Set current element value (such as text box value) via `.bval this is new value`

For example, to conduct a Google search:
1. Go to google: `.bg google.com`
2. Find search text box, an element with `name="q"`: `.bnn 10`, repeat till the search box is in sight.
3. Use `.bp` (previous) and `.bn` (next) to navigate precisely onto the search text box.
4. Type search term: `.bval this is search term`
5. Press Enter key: `.benter`
6. Navigate among search result with `.bnn`, `.bp`, `.bn`.
7. Click on link of interest: `.bptr click left`
8. Continue browsing.

## Tips
- The instance port number from configuration is only for internal localhost use. They do not have to be open on your
  network firewall.
- The web service relies on PhantomJS software that has several software dependencies:
  * bzip2, expat, zlib, fontconfig.
  * Various fonts.

  You may install the software dependencies manually, or reply on [system maintenance](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance)
to automatically install the dependencies.
- There is a latest copy of PhantomJS software in [laitos source tree](https://github.com/HouzuoGuo/laitos/blob/master/extra/linux/phantomjs-2.1.1-x86_64).
