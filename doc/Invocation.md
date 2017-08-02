# Run laitos

## Acquire software

Download the latest [laitos software](https://github.com/HouzuoGuo/laitos/releases).

For advanced usage, if you wish to compile the program from source code, you will need the latest go compiler.
Clone the source code into `GOPATH` and run `go build`:

    ~/gopath/src/github.com/HouzuoGuo > git clone git@github.com:HouzuoGuo/laitos
    ~/gopath/src/github.com/HouzuoGuo/laitos > go build

laitos program and source code do not depend on third-party program or library.

## Start

Have your [configuration file](https://github.com/HouzuoGuo/laitos/wiki/Configuration) ready.

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

The following command line flags are optional:
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