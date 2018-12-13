# Daemon: system maintenance

## Introduction
The daemon regularly conducts system maintenance to ensure smooth and safe operation of your laitos server:
- Validate configuration (such as API credentials for Twitter) used by toolbox commands and HTTP handlers.
- Collect latest network stats and program logs.
- Install latest system security updates. Keep installed applications up to date.
- Clear old temporary files.
- Harden system security by disabling unused services and users.
- Check availability of external services running on TCP ports.
- Install system software that are used by some of laitos components (such as remote controlled browser session).
- On Windows, maintain system files integrity.
- On Linux, set up firewall, set system time zone, and synchronise clock.

A summary report is generated after each run and delivered to designated Email address or printed as program output.

laitos can operate with the following software managers for security updates and software installation:
- `apt-get` (Debian, Ubuntu, etc)
- `yum` (CentOS, Fedora, Redhat, ScientificLinux, etc)
- `zypper` (openSUSE, SLES, SLED, etc)
- `chocolatey` (Windows)

## Configuration
1. Construct the following JSON object and place it under JSON key `Maintenance` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
    <th>Supported Platforms</th>
</tr>
<tr>
    <td>IntervalSec</td>
    <td>integer</td>
    <td>Interval at which system maintenance routine runs.</td>
    <td>86400 - daily maintenance is good enough</td>
    <td>All</td>
</tr>
<tr>
    <td>Recipients</td>
    <td>array of strings</td>
    <td>These Email addresses will receive the maintenance summary report.</td>
    <td>(Not used and print report as output)</td>
    <td>All</td>
</tr>
<tr>
    <td>CheckTCPPorts</td>
    <td>array of "host:ip" strings</td>
    <td>Check that these TCP ports are open on their corresponding host during maintenance routine.</td>
    <td>(Not used)</td>
    <td>All</td>
</tr>
<tr>
    <td>BlockSystemLoginExcept</td>
    <td>array of user name strings</td>
    <td>Disable Linux/Windows system users from logging in, except the names listed here.</td>
    <td>(Not used)</td>
    <td>All</td>
</tr>
<tr>
    <td>DisableStopServices</td>
    <td>array of system service name strings</td>
    <td>Disable Linux/Windows system services by stopping them and preventing them from starting.</td>
    <td>(Not used)</td>
    <td>All</td>
</tr>
<tr>
    <td>EnableStartServices</td>
    <td>array of system service name strings</td>
    <td>Enable Linux/Windows system services by starting them and letting them start at boot time.</td>
    <td>(Not used)</td>
    <td>All</td>
</tr>
<tr>
    <td>InstallPackages</td>
    <td>array of software name strings</td>
    <td>Install and upgrade these Linux/Windows software applications.</td>
    <td>(Not used)</td>
    <td>All</td>
</tr>
<tr>
    <td>BlockPortsExcept</td>
    <td>array of port numbers</td>
    <td>Set up Linux firewall to block incoming traffic to all TCP and UDP ports except those listed here.</td>
    <td>(Not used)</td>
    <td>Linux</td>
</tr>
<tr>
    <td>ThrottleIncomingPackets</td>
    <td>integer</td>
    <td>Set up Linux firewall to block flood of incoming TCP connections and UDP packets to this threshold (5 < threshold < 256).</td>
    <td>(Not used)</td>
    <td>Linux</td>
</tr>
<tr>
    <td>SetTimeZone</td>
    <td>time zone name string</td>
    <td>Set Linux system global time zone to this zone name (e.g. "Europe/Helsinki").</td>
    <td>(Not used)</td>
    <td>Linux</td>
</tr>
<tr>
    <td>TuneLinux</td>
    <td>true/false</td>
    <td>Automatically tune Linux kernel parameters for optimal performance.</td>
    <td>(Not used) false</td>
    <td>Linux</td>
</tr>
<tr>
    <td>SwapFileSizeMB</td>
    <td>integer</td>
    <td>
        Set up a Linux swap file of the specified size at /laitos-swap-file and activate it.<br />
        If it is 0, then nothing will be done about system swap.<br/>
        If it is minus, then system swap will be entirely disabled, enhancing data security.
    </td>
    <td>(Not used)</td>
    <td>Linux</td>
</tr>
<tr>
    <td>PreScriptWindows</td>
    <td>string</td>
    <td>Run this PowerShell script text prior to all other maintenance actions.</td>
    <td>(Not used)</td>
    <td>Windows</td>
</tr>
<tr>
    <td>PreScriptUnix</td>
    <td>string</td>
    <td>Run this bourne-shell script text prior to all other maintenance actions.</td>
    <td>(Not used)</td>
    <td>Linux</td>
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
The system maintenance daemon runs for the first time after a 60 seconds delay. Afterwards it runs at regular interval
specified in configuration. Manual action is not required.

## Tips
System maintenance does not have to run too often. Let it run daily is usually good enough.

The additional softwares installed and updated during maintenance are:
- Dependencies of PhantomJS used by [remote browser (PhantomJS)](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-remote-browser-(PhantomJS))
  and [text-based interactive web browser (PhantomJS)](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-interactive-web-browser-(PhantomJS)).
  They may not function properly until system maintenance has run for the first time.
- Docker container runtime and tools used by [remote browser (SlimerJS)](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-remote-browser-(SlimerJS))
  and [text-based interactive web browser (SlimerJS)](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-interactive-web-browser-(SlimerJS)).
  They may not function properly until system maintenance has run for the first time.
- Clock synchronisation tools.
- Zip file manipulation tools.
- Network and system diagnosis tools.

Exercise extra care when using the advanced maintenance options:
- Use `InstallPackages` configuration option to keep your productivity software applications up-to-date.
- Use `DisableStopServices` to disable unused system services (such as "nfs", "snmp") to save system resources.
- Use `EnableStartServices` to ensure that essential services (such as "sshd") remain active.
- Use `BlockSystemLoginExcept` to ensure that only essential users (such as "root" and "my-own-username") may login to
  the system, and all other users are blocked from login.
- Use `SetTimeZone` to set system global time zone (via changing `/etc/localtime` link). List of all available names can
  be found under directory `/usr/share/zoneinfo`.

Exercise extra care when using Linux firewall setup options:
- Use `BlockPortsExcept` to block unnecessary incoming TCP/UDP network traffic. Localhost and ICMP are not restricted.
- Keep in mind to specify port 22 (SSH) in the exception list if you are administrating Linux server remotely.
- Use `ThrottleIncomingPackets` to restrict maximum number of incoming TCP connections and UDP packets per remote IP.
