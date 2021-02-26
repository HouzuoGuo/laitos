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
- Daemons - web/mail/DNS servers, chat bots, etc. Many daemons offer access to apps, protected with a password.
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
  * [`serialport`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-serial-port-communicator) - Serial port communicator that runs app commands
  * [`simpleipsvcd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-simple-IP-services) - Simple IP services that were popular in the nostalgia era of Internet
  * [`smtpd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-mail-server) - Mail server that forwards all received Emails to your personal addresses
  * [`snmpd`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-SNMP-server) - Network management server that serves laitos program statistics
  * [`telegram`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telegram-chat-bot) - Telegram messenger chat bot that runs app commands
  * [`phonehome`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry) - Send telemetry reports of this computer to your laitos servers
  * [`plainsocket`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telnet-server) - Telnet-compatible server that runs app commands
  * [`maintenance`](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance) - Automated server maintenance and program health report
- Apps are enabled automatically once they are configured in the JSON file. Some apps such as the RSS News Reader are automatically enabled via their built-in default configuration.

## Deploy on cloud
laitos runs well on all popular cloud vendors, it supports cloud virtual machines for a straight-forward installation,
as well as more advanced cloud features such as AWS Elastic Beanstalk and AWS Lambda (in combination with API gateway).
Check out the [cloud deployment tips](https://github.com/HouzuoGuo/laitos/wiki/Cloud-tips).

## Deploy on Windows
laitos is well tuned for running on Windows server and desktop. Check out this [PowerShell script](https://raw.githubusercontent.com/HouzuoGuo/laitos/master/extra/windows/setup.ps1)
that helps to start laitos automatically as a background service.

## Advanced behaviours
### Self-healing
laitos is extremely reliable thanks to its many built-in mechanisms that activate automatically in the unlikely event of program anormaly.
The mechanisms are fully automatic and do not require manual intervention:

1. Access to external resources, such as API services on the public Internet, automatically recover from transient errors.
2. Each daemon automatically restarts in case of a transient initialisation error, such as when required system resource is unavailable.
3. laitos program and its daemons restart automatically in case of a complete program crash.
4. In the extremely unlikely event of repeated program crashes in short succession (20 minutes), laitos will restart automatically while
   shedding of its daemons starting from the heaviest daemon, thus ensuring the maximum availabily of remaining healthy daemons.

Optionally, laitos can send server owner a notification mail when a program crash occurs. To enable the notification, follow
[outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration) and then specify Email recipients in
program JSON configuration:

    {
      ...

      "SupervisorNotificationRecipients": [
        "server-owner@hotmail.com"
      ],

      ...
    }

Please use [Github issues](https://github.com/HouzuoGuo/laitos/issues) to report program crashes. Notification mail content and program
output contain valuable clues for diagnosis - please retain them for an issue report.

### More command line options
Use the following command line options with extra care:
<table>
<tr>
    <th>Flag</th>
    <th>Value data type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>-debug</td>
    <td>true/false</td>
    <td>Print stack traces to standard error upon receiving the interrupt signal SIGINT.</td>
</tr>
<tr>
    <td>-gomaxprocs Num</td>
    <td>Integer</td>
    <td>Specify maximum number of concurrent goroutines. The default value is the number of CPU cores/threads.</td>
</tr>
<tr>
    <td>-disableconflicts</td>
    <td>true/false</td>
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
<tr>
    <td>-awslambda</td>
    <td>true/false</td>
    <td>
      Launch laitos as a handler for AWS Lambda function.
      <br/>
      See <a href="https://github.com/HouzuoGuo/laitos/wiki/Cloud-tips">cloud deployment tips</a> for the detailed usage.
    </td>
</tr>
<tr>
    <td>-awsinteg</td>
    <td>true/false</td>
    <td>
      The master switch for turning on all points of integration with AWS infrastructure resources such as S3, SNS, SQS, Kinesis Firehose.
      <br/>
      See <a href="https://github.com/HouzuoGuo/laitos/wiki/Cloud-tips">cloud deployment tips</a> for the detailed usage.
    </td>
</tr>
<tr>
    <td>-prominteg</td>
    <td>true/false</td>
    <td>
      The master switch for turning on all points of integration with prometheus metrics exporter.
      <br/>
      See <a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-prometheus-metrics-exporter">Web service - prometheus metrics exporter</a>
      for the detailed usage.
    </td>
</tr>
<tr>
    <td>-profhttpport PORT</td>
    <td>Integer</td>
    <td>
      Start an HTTP server on localhost:PORT to serve program profiling data at URL location "/debug/pprof/{cmdline,profile,symbol,trace}".
    </td>
</tr>
</table>

### Build a container image
Images of a (usually) up-to-date version of laitos are uploaded to Docker Hub [hzgl/laitos](https://hub.docker.com/r/hzgl/laitos).

If you wish to customise the image to your needs, feel free to use the [`Dockerfile`](https://github.com/HouzuoGuo/laitos/blob/master/Dockerfile)
from GitHub repository as a reference.

### Supply program configuration in an environment variable
Usually, the program configuration is kept in a JSON file, the path of which is specified in the laitos launch command line (`-config my-laitos-config.json`).
However, if the program configuration is short enough to fit into an environment variable, laitos can also get its configuration
from there. This can be rather useful for testing a configuration snippet or starting a small number of daemons in a container.

The following example starts the HTTP daemon (without TLS) on the default port number (80), the web server comes with app-command runner endpoint:

    sudo env 'LAITOS_CONFIG={"HTTPFilters": {"PINAndShortcuts": {"Passwords": ["abcdefgh"]},"LintText": {"MaxLength": 1000}},"HTTPHandlers": {"AppCommandEndpoint": "/cmd"}}' ./laitos -daemons insecurehttpd
    # Try running an app command: curl http://0.0.0.0:80/cmd?cmd=abcdefgh.sdate
