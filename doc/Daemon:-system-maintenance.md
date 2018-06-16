# Daemon: system maintenance

## Introduction
The daemon regularly conducts system maintenance to ensure smooth and safe operation of your laitos server:
- Validate configuration (such as API credentials for Facebook/Twitter) used by toolbox commands and HTTP handlers.
- Collect latest network stats and program logs.
- Tune Linux kernel parameters for optimal performance.
- Install latest security updates.
- Install non-essential dependencies and diagnostic programs.
- Synchronise system clock.
- Check network ports.
- Turn off swap, disable unused system login and services for enhanced security.
- Set system time zone.

A summary report is generated after each run and delivered to designated Email address or printed as program output.

laitos can operate with the following Linux package managers for security updates and software installation:
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
    <td>(Not used and print report as output)</td>
</tr>
<tr>
    <td>CheckTCPPorts</td>
    <td>array of "host:ip" strings</td>
    <td>Check that these TCP ports are open on their corresponding host during maintenance routine.</td>
    <td>(Not used)</td>
</tr>
<tr>
    <td>BlockSystemLoginExcept</td>
    <td>array of Unix user name strings</td>
    <td>Disable these users by preventing them from logging in.</td>
    <td>(Not used)</td>
</tr>
<tr>
    <td>DisableStopServices</td>
    <td>array of system service name strings</td>
    <td>Disable these system services by stopping them and preventing them from starting.</td>
    <td>(Not used)</td>
</tr>
<tr>
    <td>EnableStartServices</td>
    <td>array of system service name strings</td>
    <td>Enable these system services by starting them and letting them start at boot time.</td>
    <td>(Not used)</td>
</tr>
<tr>
    <td>InstallPackages</td>
    <td>array of software package name strings</td>
    <td>Install and upgrade these system software packages.</td>
    <td>(Not used)</td>
</tr>
<tr>
    <td>SetTimeZone</td>
    <td>time zone name string</td>
    <td>Set system global time zone to this zone name (e.g. Europe/Helsinki).</td>
    <td>(Not used)</td>
</tr>
<tr>
    <td>TuneLinux</td>
    <td>true/false</td>
    <td>Automatically tune Linux kernel parameters for optimal performance.</td>
    <td>(Not used) false</td>
</tr>
<tr>
    <td>SwapOff</td>
    <td>true/false</td>
    <td>Turn off system swap for enhanced program security.</td>
    <td>(Not used) false</td>
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
The system maintenance daemon runs for the first time after a 2 minutes delay. Afterwards it runs at regular interval
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

Use the advanced maintenance options with extra care:
- Use `InstallPackages` configuration option to keep your productivity software packages up-to-date.
- Use `DisableStopServices` to disable unused system services (such as "nfs", "samba") to save system resources.
- Use `EnableStartServices` to ensure that essential services (such as "sshd") remain active.
- Use `BlockSystemLoginExcept` to ensure that only essential users (such as "root" and "my-own-username") may login to
  the system, and all other users are blocked from login.
- Use `SetTimeZone` to set system global time zone (via changing `/etc/localtime` link). List of all available names can
  be found under directory `/usr/share/zoneinfo`.