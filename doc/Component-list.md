# Component list

The laitos program offers a large selection of components for you to explore! A
program component belongs to one of the three categories:

- Apps - reading news and Emails, make a Tweet, ask about weather, etc.
  * Some apps do not require manual configuration and they are pre-enabled.
- Daemons - web/mail/DNS servers, chat bots, etc. Many daemons are capable of
  accepting app command input and allow command execution protected by a
  password.
- Web services - HTML-based utilities, web-hooks for integration with 3rd party
  services, etc.

Also, check out [laitos terminal](https://github.com/HouzuoGuo/laitos/wiki/Laitos-terminal)
for a terminal-UI of some of the apps.

## Daemons

<table>
    <tr>
        <th>Name</th>
        <th>Description</th>
        <th>Configuration and Usage</th>
    </tr>
    <tr>
        <td>DNS server</td>
        <td>DNS server offers a safer and cleaner web experience by blocking advertising and malicious domains.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>DNS server (invoke app commands)</td>
        <td>Included in the DNS server - execute app commands via exchanging TXT records.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server-(invoke-app-commands)" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>DNS server (TCP-over-DNS)</td>
        <td>Included in the DNS server - tunneling TCP traffic over DNS queries.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server-(TCP-over-DNS)" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Mail server</td>
        <td>Mail server forwards incoming emails to your personal email address.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-mail-server" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Web server</td>
        <td>Web server hosts a static personal website made of text and media files, along with rich web services (see below).</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Web proxy server</td>
        <td>The general purpose web proxy server is capable of handling both HTTP and HTTPS destinations.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-proxy" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Telnet server</td>
        <td>Telnet server provides unencrypted access to all apps via basic tools such HyperTerminal.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telnet-server" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Telegram messenger chat-bot</td>
        <td>Telegram chatbot provides access to all apps via secure infrastructure provided by Telegram Messenger.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telegram-chat-bot" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Simple IP services server</td>
        <td>Simple IP services were used in the nostalgic era of computing.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-simple-IP-services" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>SNMP server</td>
        <td>SNMP server offers program statistics over industrial-standard network monitoring protocol.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-SNMP-server" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>System maintenance</td>
        <td>Periodic maintenance patches the system for security updates, and checks for environment and program health.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Phone home telemetry</td>
        <td>Periodically report the system status of this computer to your laitos servers.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry" target="_blank">Link</a></td>
    </tr>
</table>

## Web services

The web services are hosted by the web server. When enabled, they will be
reachable via both HTTP and HTTPS - if the web server has a TLS certificate.

<table>
    <tr>
        <th>Name</th>
        <th>Description</th>
        <th>Configuration and Usage</th>
    </tr>
    <tr>
        <td>Twilio telephone/SMS hook</td>
        <td>Run app commands on telephone, SMS, satellite terminals via Twilio platform (telephone and SMS programming).</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Microsoft chat bot hook</td>
        <td>Run app commands via Skype and Cortana using Microsoft Bot Framework.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Microsoft-bot-hook" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>The Things Network LORA tracker integration</td>
        <td>Collect location telemetry from your LoRa IoT devices that run The Things Network Mapper program.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-the-things-network-LORA-tracker-integration" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Recurring commands</td>
        <td>Run app commands at regular interval, and retrieve their result.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-recurring-commands" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>App command form</td>
        <td>Run app commands via a web form.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-invoke-app-command" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Simple app command execution API</td>
        <td>A command-line friendly API for executing app commands.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-simple-app-command-execution-API" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>GitLab browser</td>
        <td>List and download files from your Git projects.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-GitLab-browser" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Temporary file storage</td>
        <td>Upload files for unlimited retrievel within 24 hours.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-temporary-file-storage" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Simple web proxy</td>
        <td>Let laitos download web page and send to your browser.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-simple-proxy" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Desktop on a page (virtual machine)</td>
        <td>Present you with a fully functional computer desktop running on laitos server as a virtual machine.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-desktop-on-a-page-(virtual-machine)" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Read telemetry records</td>
        <td>Read phone-home telemetry records collected by this server.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-read-telemetry-records" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Program health report</td>
        <td>Display program stats, log entries, and system resource usage in a comprehensive report.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-program-health-report" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>System process explorer</td>
        <td>Find all processes running on the host OS and inspect the status and resource usage of individual process.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-system-process-explorer" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Prometheus metrics exporter</td>
        <td>Serve metrics info collected from web server, web proxy server, program resource usage, in prometheus exporter format.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-prometheus-metrics-exporter" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>HTTP request inspector</td>
        <td>Inspect the incoming HTTP request by repeating it back in the response.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-request-inspector" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>HTTP request logger</td>
        <td>Log all incoming HTTP request for inspection.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-request-logger" target="_blank">Link</a></td>
    </tr>
</table>

## Apps

<table>
    <tr>
        <th>Name</th>
        <th>Description</th>
        <th>Configuration and Usage</th>
    </tr>
    <tr>
        <td>Use Twitter</td>
        <td>Read and post tweets.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-Twitter" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Ask WolframAlpha</td>
        <td>Ask about weather and all sorts of questions on WolframAlpha - the computational knowledge engine.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-WolframAlpha" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>RSS feeds</td>
        <td>Read news feeds and briefings via RSS.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-RSS-reader" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Wild joke</td>
        <td>Grab a quick laugh from the Internet.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-wild-joke" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Read Emails</td>
        <td>Read your personal Emails from popular sites such as Hotmail and Gmail.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-reading-emails" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Send Emails</td>
        <td>Send Emails to friends, and send SOS emails in situations of distress.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-sending-emails" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Make calls and send SMS</td>
        <td>Send text to friend's phone number, or call a friend to speak a short message.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-make-calls-and-send-SMS" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>2FA code generator</td>
        <td>Generate two-factor authentication codes.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-two-factor-authentication-code-generator" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Password book</td>
        <td>Decrypt AES-encrypted files (e.g. password book) and search for keywords among the content.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-find-text-in-AES-encrypted-files" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Text search</td>
        <td>Search for keywords among text files such as telephone book.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-text-search" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Public contacts</td>
        <td>Look up contact information from several public institutions.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-public-institution-contacts" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Run system commands</td>
        <td>Run Linux/Unix shell commands on laitos server.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-run-system-commands" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Program control</td>
        <td>Retrieve laitos server environment information, and self-destruct in unfortunate moments.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-inspect-and-control-server-environment" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Phone home telemetry handler</td>
        <td>Read telemetry record fields from input and store them in memory.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-phone-home-telemetry-handler" target="_blank">Link</a></td>
    </tr>
</table>
