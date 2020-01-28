# Component list

The rich set of components not only covers the basic needs of hosting a personal web server,
but also provide advanced capabilities to satisfy the geeky nature inside of you!

laitos components go into three categories:
- Apps - reading news and Emails, make a Tweet, ask about weather, etc.
- Daemons - web/mail/DNS servers, chat bots, etc. Many daemons offer access to apps, protected with a password PIN.
- Rich web services - useful web-based utilities hosted by the web server.

## Daemons
<table>
    <tr>
        <th>Name</th>
        <th>Description</th>
        <th>Configuration and Usage</th>
    </tr>
    <tr>
        <td>DNS server</td>
        <td>DNS server offers a safer and cleaner web experience by blocking advertising and malware domains.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server" target="_blank">Link</a></td>
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
        <td>System maintenance</td>
        <td>Periodic maintenance patches the system for security updates, and checks for environment and program health.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Telnet server</td>
        <td>Telnet server provides unencrypted access to all apps via basic tools such HyperTerminal.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telnet-server" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Serial port communicator</td>
        <td>Serial port communicator provides access to all apps to serial port devices.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-serial-port-communicator" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>SNMP server</td>
        <td>SNMP server offers program statistics over industrial-standard network monitoring protocol.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-SNMP-server" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Simple IP services server</td>
        <td>Simple IP services were used in the nostalgic era of computing.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-simple-IP-services" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Telegram messenger chat-bot</td>
        <td>Telegram chatbot provides access to all apps via secure infrastructure provided by Telegram Messenger.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telegram-chat-bot" target="_blank">Link</a></td>
    </tr>
</table>


#### Rich web services
The following services are hosted by web server and enabled on your demand:

<table>
    <tr>
        <th>Name</th>
        <th>Description</th>
        <th>Configuration and Usage</th>
    </tr>
    <tr>
        <td>Program health report</td>
        <td>Display program stats and environment info in a comprehensive report.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-program-health-report" target="_blank">Link</a></td>
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
        <td>Web browser on a page (SlimerJS)</td>
        <td>Present you with a fully functional web browser running on laitos server. It uses the newer SlimmerJS technology.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-web-browser-on-a-page-(SlimerJS)" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Web browser on a page (PhantomJS)</td>
        <td>Present you with a fully functional web browser running on laitos server. It uses the older PhantomJS technology.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-web-browser-on-a-page-(PhantomJS)" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Desktop on a page (virtual machine)</td>
        <td>Present you with a fully functional computer desktop running on laitos server as a virtual machine.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-desktop-on-a-page-(virtual-machine)" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>App command form</td>
        <td>Run app commands via a web form.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-invoke-app-command" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Twilio telephone/SMS hook</td>
        <td>Run app commands on telephone, SMS, satellite terminals via Twilio platform (telephone and SMS programming).</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Microsoft bot hook</td>
        <td>Run app commands on Skype and Cortana via Microsoft Bot Framework.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Microsoft-bot-hook" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Recurring commands</td>
        <td>Run app commands at regular interval, and retrieve their result.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-recurring-commands" target="_blank">Link</a></td>
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
        <td>Use WolframAlpha</td>
        <td>Ask about weather and all sorts of questions on WolframAlpha - the computational knowledge engine.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-WolframAlpha" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Wild joke</td>
            <td>Grab a quick laugh from the Internet.</td>
            <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-wild-joke" target="_blank">Link</a></td>
        </tr>
    <tr>
        <td>RSS feeds</td>
            <td>Read news feeds and briefings via RSS.</td>
            <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-RSS-reader" target="_blank">Link</a></td>
        </tr>
    <tr>
        <td>Web browser (PhantomJS)</td>
        <td>Take control over a fully feature web browser (PhantomJS) via text commands.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-interactive-web-browser-(PhantomJS)" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Web browser (SlimerJS)</td>
        <td>Take control over a fully feature web browser (SlimerJS) via text commands.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-interactive-web-browser-(SlimerJS)" target="_blank">Link</a></td>
        </tr>
    <tr>
        <td>Read Emails</td>
        <td>List and read personal Emails from popular services such as Hotmail and Gmail.</td>
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
        <td>Public contacts</td>
        <td>Look up contact information from several public institutions.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-public-institution-contacts" target="_blank">Link</a></td>
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
        <td>Run system commands</td>
        <td>Run Linux/Unix shell commands on laitos server.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-run-system-commands" target="_blank">Link</a></td>
    </tr>
    <tr>
        <td>Program control</td>
        <td>Retrieve laitos server environment information, and self-destruct in unfortunate moments.</td>
        <td><a href="https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-inspect-and-control-server-environment" target="_blank">Link</a></td>
    </tr>
</table>
