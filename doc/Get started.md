# Get started

## Acquire software

Download the latest [laitos software](https://github.com/HouzuoGuo/laitos/releases).

For advanced usage, use the latest go compiler to compile the software from source code like so:

    ~/gopath/src/github.com/HouzuoGuo > git clone git@github.com:HouzuoGuo/laitos
    ~/gopath/src/github.com/HouzuoGuo/laitos > go build

laitos program and source code do not depend on third-party program or library.

## Prepare configuration

laitos components go into two categories:
- Features - access to Email, post to Twitter/Facebook, etc.
- Daemons - web server, mail server, and chat bots. Daemons grant access to all features.

laitos uses a text file written in [JSON](https://en.wikipedia.org/wiki/JSON) to configure all components.
Make an empty text file and write down `{ }` - an empty JSON object, then go through the links in [Feature list](https://github.com/HouzuoGuo/laitos/wiki/Feature-list) to craft your very own configuration.

Keep in mind - a component without configuration remains inactive.

## Start program
Assume that latios software is in current directory, run the following command:

    sudo ./laitos -config <CONFIG FILE> -frontend <LIST>

Note that:
- Web, mail, and many other daemons bind to [privileged ports](https://www.w3.org/Daemon/User/Installation/PrivilegedPorts.html), therefore use `sudo` to ensure their proper operation.
- Replace `<CONFIG FILE>` by the path to your configuration file. Both absolute and relative paths are acceptable.
- Replace `<LIST>` by a comma-separated list of daemon names e.g. `dnsd,smtpd,httpd`. Here are the options:
  * dnsd - Ad-blocking DNS server
  * httpd - Web server secured by TLS certificate
  * insecurehttpd - Web server without encryption
  * mailp - Text mail processor (note the [special usage](https://github.com/HouzuoGuo/laitos/wiki/Text-mail-processor))
  * maintenance - System maintenance and health reports
  * smtpd - Mail server
  * telegram - Telegram messenger chat bot
  
## Advanced start
The following command options are optional:
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
    <td>-disableconflicts</td>
    <td>
        Automatically disable the following system softwares that may run into resource conflict:<br>
        <ul>
            <li>Apache web server</li>
            <li>Bind DNS server</li>
            <li>lighttpd web server</li>
            <li>postfix mail server</li>
            <li>sendmail mail server</li>
        </ul>
    </td>
</tr>
<tr>
    <td>-gomaxprocs</td>
    <td>Specify maximum number of concurrent goroutines. Default to number of system CPU threads (go 1.8+).</td>
</tr>
<tr>
    <td>-tunesystem</td>
    <td>Automatically optimise operating system parameters to improve program performance. Only works on Linux.</td>
</tr>
</table>

## Public cloud
For deploying on Amazon/Azure/Google cloud, check out these [tips](https://github.com/HouzuoGuo/laitos/wiki/Public-cloud).