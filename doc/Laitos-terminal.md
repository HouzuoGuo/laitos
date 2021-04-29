# Introduction

The
[laitos terminal](https://github.com/HouzuoGuo/laitos/blob/master/laitos-terminal.sh)
is a character-based user interface for using laitos apps. The features are:

-   Configure and save the app execution password PIN.
-   Select the desired app to use from the main menu.
-   Enter parameters in a form.
-   Execute using the previously configured password PIN and read the command
    response.
-   Display the reachability of your laitos server in real time.

The program also offers a "low bandwidth mode" designed specially for using
narrow-band satellite Internet connections - it works even for a 2400
bits/second satellite modem. See also
[tips for using apps over satellite](https://github.com/HouzuoGuo/laitos/wiki/Tips-for-using-apps-over-satellite).

Here is the terminal in live action (the blue window in foreground):
<img src="https://raw.githubusercontent.com/HouzuoGuo/laitos/master/doc/cosmetic/20200825-poster.png" alt="poster image" />

# Usage

## Prepare your laitos server

In order to run app commands, the terminal program contacts your
[laitos web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server)
over HTTPS, and uses the
[simple app command execution API](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-simple-app-command-execution-API).

Make sure the laitos web server is configured to start, and configure the
command execution API handler as well.

If you wish to use the "low bandwidth mode" in the terminal program, then
configure laitos web server to start the
[HTTP server (`insecurehttpd`)](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Launch the terminal program

The terminal program is written in "bash" shell scripting language and will run
on Linux, Unix, and Windows WSL. Beside "bash" (v4.0 or newer), the terminal
also uses "dialog" (v1.3 or newer), "socat" (v1.7 or newer), and "curl" (v7.61
or newer) installed. The
[laitos system maintenance](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance)
daemon can automatically install these software for you.

To install the mandatory dependencies manually on Debian/Ubuntu, run system
command `sudo apt install -y bash dialog socat curl`.

Download the program
[`laitos-terminal.sh`](https://raw.githubusercontent.com/HouzuoGuo/laitos/master/laitos-terminal.sh)
and save it to anywhere you like. Navigate to the directory where it's saved,
and start the program with `bash ./laitos-terminal.sh`

Once the program is started, you'll be greeted with "App Menu". First, let's
enter the app command password PIN;

-   Use keyboard cursor keys to navigate between menu items and buttons. Use the
    space key to mark a menu choice, and use the Enter key press the highlighted
    button.
-   Visit the "Configure laitos server address and more" to enter laitos web
    server host name/IP, app command execution API endpoint, and app command
    processor password.
-   Press Enter key on keyboard to save the configuration.

Use keyboard cursor keys to navigate to an app, press Space key to mark it, and
start using the laitos apps!

In the background, the laitos terminal program will regularly check the laitos
server connectivity and display it on the screen.

## Tips

-   The laitos terminal connects to your laitos server over HTTP (or HTTPS),
    therefore, feel free to leave unrelated port number fields blank in the app
    configuration, and they will not be probed for connection status test.
-   Be aware that your laitos DNS server configuration may restrict incoming
    queries to selected IP blocks only, the computer running laitos terminal may
    or may not be among the permitted IP blocks. Regardless, the terminal does
    not depend on your laitos DNS server.
-   In the app configuration, The "low bandwidth mode" forces usage of HTTP,
    reducing communication security to ensure that the terminal will properly
    work over a narrowband satellite Internet link.
-   The "low bandwidth mode" has been successfully tested using Inmarsat
    iSatPhone 2, which provides a narrowband 2400 bit/second data modem with a
    high-latency (2s round trip) connection. The regular bandwidth mode cannot
    work over a narrowband link.
