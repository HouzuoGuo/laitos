## Introduction
The [laitos terminal](https://github.com/HouzuoGuo/laitos/blob/master/laitos-terminal.sh) is a character-based
user interface for interacting with laitos apps. The program is written in "bash" as a shell script, and contacts
laitos web server to execute app commands.

The program also offers a "low bandwidth mode" designed specially for high-latency and narrowband satellite Internet
connection, the mode successfully connects to laitos server via a 2400 bit/s satellite modem.

## Get started
Using Linux, Unix, or Windows WSL, first make sure that it has got "bash" (v4.0 or newer), "dialog" (v1.3 or newer),
"socat" (v1.7 or newer), and "curl" (v7.61 or newer) installed. The [laitos system maintenance](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance)
daemon can automatically install these software for you.

To install the mandatory dependencies manually on Debian/Ubuntu, run system command `sudo apt install -y bash dialog socat curl`.

Download the program [`laitos-terminal.sh`](https://raw.githubusercontent.com/HouzuoGuo/laitos/master/laitos-terminal.sh)
and save it to anywhere you like. Navigate to the directory where it's saved, and start the program with `bash ./laitos-terminal.sh`

Once the program is started, you'll be greeted with "App Menu":
- Use keyboard cursor keys to navigate between menu items and buttons. Use the space key to mark a menu choice, and
  use the Enter key press the highlighted button.
- Visit the "Configure laitos server address and more" to enter laitos web server host name/IP, app command execution
  API endpoint, and app command processor password.
- Press Enter key on keyboard to save the configuration.

The laitos terminal program will regularly probe your laitos server and display the latest connection status. Use keyboard
cursor keys to navigate to an app, press Space key to mark it, and start enjoying the terminal features!

## Tips
- The laitos terminal connects to your laitos server over HTTP (or HTTPS), therefore, feel free to leave unrelated port
  number fields blank in the app configuration, and they will not be probed for connection status test.
- Be aware that your laitos DNS server configuration may restrict incoming queries to selected IP blocks only, the computer
- running laitos terminal may or may not be among the permitted IP blocks. Regardless, the terminal does not depend on your
  laitos DNS server.
- In the app configuration, The "low bandwidth mode" forces usage of HTTP, reducing communication security to ensure that
  the terminal will properly work over a narrowband satellite Internet link.
- The "low bandwidth mode" has been successfully tested using Inmarsat iSatPhone 2, which provides a narrowband 2400 bit/second
  data modem with a high-latency (2s round trip) connection. The regular bandwidth mode cannot work over a narrowband link.
