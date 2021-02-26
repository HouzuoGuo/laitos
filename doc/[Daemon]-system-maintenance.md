## Introduction
The daemon regularly carries out system maintenance to ensure smooth and safe operation of your laitos server.
A summary report is generated after each run and delivered to designated Email recipients.

System maintenance tasks comprise:

(For laitos)
- Validate configuration (such as API credentials for Twitter) used by apps and HTTP handlers.
- Collect latest daemon stats summary and collect latest log entries.
- Install common system administration and maintenance software using system package manager.
- Install dependency software of advanced laitos features (such as browser-in-a-browser and VM-in-a-browser) using
  system package manager.

(For system security)
- Install the latest system security updates and keep installed software up to date.
- Harden system security by disabling unused services and users (additional configuration required).
- Set up Linux firewall to throttle incoming packets and block unused ports (additional configuration required).

(For routine maintenance)
- Defragment drives, trim SSD drives, and delete expired temporary files.
- Synchronise system clock.
- On Windows, maintain system files integrity with `DISM` and `SFC`.
- Set Linux system time zone (additional configuration required).

(Miscellaneous)
- Perform connection check on external TCP services (additional configuration required).
- Collect laitos program resource usage metrics (such as CPU usage and scheduler performance) for the
  [prometheus metrics exporter web service](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-prometheus-metrics-exporter).

laitos works with the following system package managers for installing and updating system software:
- `apt-get` (Debian, Ubuntu, etc)
- `yum` (Amazon Linux, CentOS, RedHat, Fedora, etc)
- `dnf` (via its `yum`-compatibility)
- `zypper` (openSUSE, SLES, SLED, etc)
- `chocolatey` (Windows server & desktop)

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
    <td>Run the system maintenance routine regularly at this interval (seconds). It must be greater or equal to 86400 (24 hours).</td>
    <td>86400 - daily maintenance is good enough</td>
    <td>Universal</td>
</tr>
<tr>
    <td>Recipients</td>
    <td>array of strings</td>
    <td>These Email addresses will receive the maintenance summary report.</td>
    <td>(Not used and print report as output)</td>
    <td>Universal</td>
</tr>
<tr>
    <td>CheckTCPPorts</td>
    <td>array of "host:ip" strings</td>
    <td>Check that these TCP ports are open on their corresponding host during maintenance routine.</td>
    <td>(Not used)</td>
    <td>Universal</td>
</tr>
<tr>
    <td>BlockSystemLoginExcept</td>
    <td>array of user name strings</td>
    <td>Disable Linux/Windows system users from logging in, except the names listed here.</td>
    <td>(Not used)</td>
    <td>Universal</td>
</tr>
<tr>
    <td>DisableStopServices</td>
    <td>array of system service name strings</td>
    <td>Disable Linux/Windows system services by stopping them and preventing them from starting.</td>
    <td>(Not used)</td>
    <td>Universal</td>
</tr>
<tr>
    <td>EnableStartServices</td>
    <td>array of system service name strings</td>
    <td>Enable Linux/Windows system services by starting them and letting them start at boot time.</td>
    <td>(Not used)</td>
    <td>Universal</td>
</tr>
<tr>
    <td>InstallPackages</td>
    <td>array of software name strings</td>
    <td>Install and upgrade these Linux/Windows software applications.</td>
    <td>(Not used)</td>
    <td>Universal</td>
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
    <td>Automatically tune server kernel parameters for enhnaced performance and security.</td>
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
<tr>
    <td>UploadReportToS3Bucket</td>
    <td>string</td>
    <td>After completing a round of maintenance, upload the report of results to this AWS S3 bucket.</td>
    <td>(Not used)</td>
    <td>Universal</td>
</tr>
</table>
2. Follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration).


Here is an example configuration that keeps system up-to-date, while also checking whether mail(25), DNS(53), and HTTP(80, 443) daemons are online:
<pre>
{
    ...

    "Maintenance": {
        "Recipients": ["me@example.com"],
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


If you opt to upload maintenance reports to AWS S3 bucket, please follow the [Cloud Tips - Integrate with AWS](https://github.com/HouzuoGuo/laitos/wiki/Cloud-tips)
section to give laitos program `AWS_REGION` and access key.

## Run
Tell laitos to run periodic system maintenance in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,maintenance,...

## Usage
The daemon runs the maintenance routine 3 minutes after it starts up, afterwards it automatically runs at regular interval according to configuration.
No manual or interactive action is required.

Each run produces a very detailed system maintenance report for inspection, which may be found in:
- A text file located under system temporary files directory (`/tmp/laitos-latest-maintenance-report.txt` for Linux and `%USERPROFILE%/AppData/Local/Temp/laitos-latest-maintenance-report.txt` for Windows).
  Old report file will be overwritten.
- An Email addressed to the recipients defined in configuration.
- `laitos` program standard output (only if there are no Email recipeints).

## Tips
System maintenance does not have to run too often. Let it run daily is usually good enough.

The maintenance routine always automatically installs the following software and keeps them up-to-date. The altogether use about 300 MB of disk space:
- Dependencies of PhantomJS used by [web browser on a page (PhantomJS)](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-web-browser-on-a-page-(PhantomJS))
  and [text-based interactive web browser (PhantomJS)](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-interactive-web-browser-(PhantomJS)).
  They may not function properly until system maintenance has run for the first time.
- Docker container runtime and tools used by [web browser on a page (SlimerJS)](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-web-browser-on-a-page-(SlimerJS))
  and [text-based interactive web browser (SlimerJS)](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-interactive-web-browser-(SlimerJS)).
  They may not function properly until system maintenance has run for the first time.
- QEMU and KVM virtualisation software.
- Clock synchronisation tools.
- Other system administration and diagnosis tools.

Exercise extra care when using the advanced maintenance options:
- Use `InstallPackages` configuration option to keep your productivity software applications up-to-date.
- Use `DisableStopServices` to disable unused system services of your choice (such as "nfs", "snmp") to conserve system resources.
- Use `EnableStartServices` to ensure that essential services of your choice (such as "sshd") remain active.
- Use `BlockSystemLoginExcept` to ensure that only essential users (such as "root" and "my-own-username") may login to
  the system, and all other users are blocked from login.
- Use `SetTimeZone` to set system global time zone (via changing `/etc/localtime` link). List of all available names can
  be found under directory `/usr/share/zoneinfo`.

Exercise extra care when using Linux firewall setup options:
- Use `BlockPortsExcept` to block unnecessary incoming TCP/UDP network traffic. Localhost and ICMP are not restricted.
- Keep in mind to specify port 22 (SSH) in the exception list if you are administrating Linux server remotely.
- Use `ThrottleIncomingPackets` to restrict maximum number of incoming TCP connections and UDP packets per remote IP.
