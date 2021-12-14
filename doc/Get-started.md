# Get started

## Obtain laitos software

Download the latest laitos software from the [releases page](https://github.com/HouzuoGuo/laitos/releases).

Alternatively, compile the software manually by cloning this repository and then
run `go build`.

## Craft your configuration

A configurable laitos component belongs to one of the three categories:

- Apps - reading news and Emails, make a Tweet, ask about weather, etc.
  * Some apps do not require manual configuration and they are pre-enabled.
- Daemons - web/mail/DNS servers, chat bots, etc. Many daemons are capable of
  accepting app command input and allow command execution protected by a
  password.
- Web services - HTML-based utilities, web-hooks for integration with 3rd party
  services, etc.

Follow the links in [component list](https://github.com/HouzuoGuo/laitos/wiki/Component-list)
to craft your very own configuration in [JSON](https://en.wikipedia.org/wiki/JSON).

As an example, here we use laitos DNS server to provide a safer and ad-free web
experience at home, and enable a couple of web utilities:

    {
      "DNSDaemon": {
        "AllowQueryIPPrefixes": [
          "192.",
          "10."
        ]
      },
      "HTTPDaemon": {},
      "HTTPHandlers": {
        "CommandFormEndpoint": "/cmd",
        "FileUploadEndpoint": "/upload",
        "InformationEndpoint": "/info",
        "LatestRequestsInspectorEndpoint": "/latest_requests",
        "ProcessExplorerEndpoint": "/proc",
        "RequestInspectorEndpoint": "/myrequest",
        "WebProxyEndpoint": "/proxy"
      }
    }

## Start the program

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

## Other deployment techniques

### Use environment variables to feed the program configuration

For ease of deployment, laitos can fetch its program configuration along with
the content of HTTP daemon index page from environment variables - instead of
the usual files.

When `LAITOS_CONFIG` environment variable is present and not empty, laitos
program will load its configuration from there. When `LAITOS_INDEX_PAGE`
environment variable is present and not empty, laitos will use its content
to serve the index page on its HTTP servers.

Check out environment variable usage examples in the [Kubernetes example](https://github.com/HouzuoGuo/laitos/blob/master/k8s.example/laitos-in-k8s.yaml)
and the [example in Dockerfile](https://github.com/HouzuoGuo/laitos/blob/master/Dockerfile)

Be aware that the combined size of all environment variables generally cannot
exceed ~2MBytes.

### Build a container image
The images of a (usually) up-to-date version of laitos are uploaded to Docker
Hub [hzgl/laitos](https://hub.docker.com/r/hzgl/laitos).

If you wish to customise the image to your needs, feel free to use the [`Dockerfile`](https://github.com/HouzuoGuo/laitos/blob/master/Dockerfile)
from GitHub repository as a reference.

## Deploy on cloud

laitos runs well on all popular cloud vendors, it supports cloud virtual machines for a straight-forward installation,
as well as more advanced cloud features such as AWS Elastic Beanstalk and AWS Lambda (in combination with API gateway).
Check out the [cloud deployment tips](https://github.com/HouzuoGuo/laitos/wiki/Cloud-tips).

## Deploy on Windows

laitos is well tuned for running on Windows server and desktop. Check out this [PowerShell script](https://raw.githubusercontent.com/HouzuoGuo/laitos/master/extra/windows/setup.ps1)
that helps to start laitos automatically as a background service.

## Advanced program behaviours

### Self-healing

laitos is extremely reliable thanks to its many built-in mechanisms that make
automated attempts to restart and isolate faulty components. The built-in
mechanisms are fully automatic and do not require intervention:

1. Automatically recover from transient errors when contacting external
   resources, such as API services on the public Internet.
2. Every daemon automatically restarts in case of a transient initialisation
   error.
3. In the unlikely event of a program crash, the laitos program automatically
   restarts itself to recover.
4. In the extremely unlikely event of repeated program crashes in short
   succession (20 minutes), laitos will attempt to automatically isolate the
   faulty daemon by removing daemons before the next restart - shedding the
   heavier daemons (e.g. DNS) first before shedding the lighter daemons (e.g.
   HTTP daemon).

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
