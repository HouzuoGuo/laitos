# Get started

## Acquire software

Download the latest [laitos software](https://github.com/HouzuoGuo/laitos/releases).

For advanced usage, use the latest go compiler to compile the software from source code like so:

    ~/gopath/src/github.com/HouzuoGuo > git clone https://github.com/HouzuoGuo/laitos.git
    ~/gopath/src/github.com/HouzuoGuo/laitos > go build

laitos program and source code do not depend on third-party program or library.

## Prepare configuration

laitos components go into two categories:
- Toolbox features - access to Email, post to Twitter/Facebook, etc.
- Daemons - web server, mail server, and chat bots. Daemons grant access to all toolbox features.

laitos uses a text file written in [JSON](https://en.wikipedia.org/wiki/JSON) to configure all components.
Make an empty text file and write down `{ }` - an empty JSON object, then go through the links in [component list](https://github.com/HouzuoGuo/laitos/wiki/Component-list)
to craft your very own configuration.

Keep in mind - a component without configuration remains inactive.

## Start program
Assume that latios software is in current directory, run the following command:

    sudo ./laitos -config <CONFIG FILE> -frontend <LIST>

Note that:
- Web, mail, and many other daemons usually bind to [privileged ports](https://www.w3.org/Daemon/User/Installation/PrivilegedPorts.html),
  use `sudo` to ensure their proper operation.
- Replace `<CONFIG FILE>` by the path to your configuration file. Both absolute and relative paths are acceptable.
- Replace `<LIST>` by name of daemons to start. Use comma to separate names (e.g.`dnsd,smtpd,httpd`). Here are the options:
  * `dnsd` - Ad-blocking DNS server
  * `httpd` - Web server secured by TLS certificate
  * `insecurehttpd` - Web server without TLS encryption
  * `mailcmd` - Text mail processor (note the [special usage](https://github.com/HouzuoGuo/laitos/wiki/STDIN-mail-processor))
  * `maintenance` - System maintenance and health reports
  * `plainsocket` - Access to toolbox features via TCP/UDP in plain text
  * `smtpd` - Mail server
  * `telegram` - Telegram messenger chat bot
- There is not any individual ON-OFF switch for toolbox features. Once configured, they are are automatically available to daemons.
  
## Advanced start
The following command options are optional, use with care:
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
            <li>apache web server</li>
            <li>bind DNS server</li>
            <li>lighttpd web server</li>
            <li>postfix mail server</li>
            <li>sendmail mail server</li>
        </ul>
    </td>
</tr>
<tr>
    <td>-swapoff</td>
    <td>Turn off swap files and partitions on the system for improved system security.</td>
</tr>
<tr>
    <td>-gomaxprocs</td>
    <td>Specify maximum number of concurrent goroutines. Default to number of system CPU threads.</td>
</tr>
<tr>
    <td>-tunesystem</td>
    <td>Automatically optimise operating system parameters to improve program performance.</td>
</tr>
</table>

## Public cloud
For deploying on Amazon/Azure/Google cloud, check out these [tips](https://github.com/HouzuoGuo/laitos/wiki/Public-cloud).