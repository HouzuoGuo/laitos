# Daemon: system maintenance

## Introduction
The daemon regularly conducts system maintenance to ensure smooth and safe operation of your laitos server:
- Validate API credentials (Facebook, Twitter, chatbot, etc) used by toolbox commands and HTTP handlers.
- Install latest security updates.
- Install/update additional softwares that are useful to laitos' operation.
- Synchronise system clock.
- Check network ports.
- Collect latest network stats and program logs.

At end of each run, your will receive a nicely formatted summary report via mail.

It is compatible with the following Linux package managers for security updates and software installation:
- `apt-get` (Debian, Ubuntu, etc)
- `yum` (CentOS, Fedora, Redhat, ScientificLinux, etc)
- `zypper` (openSUSE, SLES, SLED, etc)

## Configuration
1. Construct the following JSON object and place it under JSON key `Maintenance` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>IntervalSec</td>
    <td>integer</td>
    <td>Interval at which system maintenance routine runs.</td>
    <td>86400 - daily maintenance is good enough</td>
</tr>
<tr>
    <td>Recipients</td>
    <td>array of strings</td>
    <td>These Email addresses will receive the maintenance summary report.</td>
    <td>(Not used)</td>
</tr>
<tr>
    <td>CheckTCPPorts</td>
    <td>array of "host:ip" strings</td>
    <td>Check that these TCP ports are open on their corresponding host during maintenance routine.</td>
    <td>(Not used)</td>
</tr>
</table>
2. Follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration).

Here is an example configuration that also checks whether mail(25), DNS(53), and HTTP(80, 443) daemons are working:
<pre>
{
    ...

    "Maintenance": {
        "Recipients": ["howard@gmail.com"],
        "CheckTCPPorts": [
            "localhost:25",
            "localhost:53",
            "localhost:80",
            "localhost:443"
        ]
    },

    ...
}
</pre>

## Run
Tell laitos to run periodic system maintenance in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,maintenance,...

## Usage
System maintenance is run automatically 10 minutes after laitos daemon starts up, and then at regular interval specified
in configuration.

Manual action is not required.

## Tips
System maintenance does not have to run too often. Let it run daily is usually good enough.

The additional softwares installed during maintenance are:
- Dependencies of PhantomJS, used by [browser-in-browser](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-browser-in-browser)
  and [text-based interactive web browser](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-interactive-web-browser).
  The two features may not function properly until system maintenance has run for the first time.
- Clock synchronisation tools.
- Zip file manipulation tools.
- Network and system diagnosis tools.