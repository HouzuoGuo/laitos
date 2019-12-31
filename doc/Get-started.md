# Get started

## Acquire software
Download the latest [laitos software](https://github.com/HouzuoGuo/laitos/releases).

For geekier scenarios, use the latest go compiler to compile the software from source code like so:

    ~/go > go get github.com/HouzuoGuo/laitos
    ~/go/src/github.com/HouzuoGuo/laitos > go build

laitos is an all-in-one solution and does not depend on third party library.

## Prepare configuration
laitos components go into three categories:
- Apps - reading news and Emails, make a Tweet, ask about weather, etc.
- Daemons - web/mail/DNS servers, chat bots, etc. Many daemons offer access to apps, protected with a password PIN.
- Rich web services - useful web-based utilities hosted by the web server.

Follow the links in [component list](https://github.com/HouzuoGuo/laitos/wiki/Component-list) to craft your very own
configuration in [JSON](https://en.wikipedia.org/wiki/JSON).
Keep in mind - nearly all components require configuration to be useful.

As an example, here we use laitos DNS server for a safer and ad-free web experience at home, and automatically keep
the laitos server computer up-to-date with latest security patches:

    {
      "DNSDaemon": {
        "AllowQueryIPPrefixes": [
          "192.",
          "10."
        ]
      },
      "Maintenance": {
        "Recipients": [
          "server-owner@hotmail.com"
        ]
      },
    }

## Start program
Assume that latios software is in current directory, run the following command:

    sudo ./laitos -config <PATH TO JSON FILE> -daemons <LIST>

Note that:
- Web, mail, and many other daemons usually bind to [privileged ports](https://www.w3.org/Daemon/User/Installation/PrivilegedPorts.html),
  Run laitos using `sudo` to ensure their proper operation.
- Replace `<PATH TO JSON FILE>` by the relative or absolute path to your configuration file.
- Replace `<LIST>` by daemon names to start. Use comma to separate names (e.g.`dnsd,smtpd,httpd`). Here are the names:
  * [`dnsd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server) - DNS server for ad-free and safer browsing experience
  * [`httpd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server) - Web server secured by TLS certificate
  * [`insecurehttpd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server) - Web server without TLS encryption
  * [`serialport`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-serial-port-communicator) - Serial port communicator
  * [`simpleipsvcd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-simple-IP-services) - Simple IP services
  * [`smtpd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-mail-server) - Mail server
  * [`snmpd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-SNMP-server) - Network management (program statistics) server
  * [`telegram`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telegram-chat-bot) - Telegram messenger chat bot
  * [`plainsocket`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telnet-server) - Use plain text (Telnet) over TCP and UDP to access apps.
  * [`maintenance`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance) - Automated server maintenance and program health report
- Apps are enabled automatically once they are configured in the JSON file. Some apps such as the RSS News Reader are automatically enabled via their built-in default configuration.

## Deploy on cloud
laitos runs well on all popular cloud vendors. Check out these [tips](https://github.com/HouzuoGuo/laitos/wiki/Cloud-tips)
for smoother deployment experience.

## Deploy on Windows
laitos is well tuned for running on Windows server and desktop. Check out this [PowerShell script](https://raw.githubusercontent.com/HouzuoGuo/laitos/master/extra/windows/setup.ps1)
that helps to start laitos automatically as a background service.

## Advanced behaviours
### Supervisor
laitos uses a built-in supervisor mechanism to restart main program in the unlikely event of crash. If under extremely
rare circumstances laitos crashes more than once in quick succession (20 minutes), the supervisor will restart main
program while shedding off potentially faulty components.

Optionally, the supervisor can send server owner a notification mail when a crash occurs. To enable the notification,
follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration) and then
specify recipients in program JSON configuration:

    {
      ...

      "SupervisorNotificationRecipients": [
        "server-owner@hotmail.com"
      ],

      ...
    }

Please use [Github issues](https://github.com/HouzuoGuo/laitos/issues) to report laitos crashes. Notification mail
content and program output contain valuable clues for diagnosis.

### More command line options
Use the following command line options with extra care:
<table>
<tr>
    <th>Flag</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>-debug</td>
    <td>Print stack traces to standard error upon receiving the interrupt signal SIGINT.</td>
</tr>
<tr>
    <td>-gomaxprocs</td>
    <td>Specify maximum number of concurrent goroutines. Default to number of system CPU threads.</td>
</tr>
<tr>
    <td>-disableconflicts</td>
    <td>
        Automatically disable the following system softwares that may run into resource conflict:<br>
        <ul>
            <li>apache web server</li>
            <li>bind DNS server</li>
            <li>systemd-resolved DNS proxy</li>
            <li>lighttpd web server</li>
            <li>postfix mail server</li>
            <li>sendmail mail server</li>
        </ul>
    </td>
</tr>
</table>
