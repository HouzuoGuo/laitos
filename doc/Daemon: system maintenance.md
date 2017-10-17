# Daemon: system maintenance

## Introduction
The daemon regularly conducts system maintenance to ensure smooth and safe operation of your laitos server:
- Install latest security updates.
- Install/update additional softwares that are useful to laitos' operation.
- Synchronise system clock.
- Check network ports.
- Collect latest network stats and program logs.

At end of each run, your Email address will receive a nicely formatted summary report.

It is compatible with the following Linux package managers for security updates and software installation:
- `apt-get` (Debian, Ubuntu, etc)
- `yum` (CentOS, Fedora, Redhat, ScientificLinux, etc)
- `zypper` (openSUSE, SLES, SLED, etc)

## Configuration
Construct the following JSON object and place it under JSON key `Maintenance` in configuration file. The following
properties are mandatory:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>IntervalSec</td>
    <td>integer</td>
    <td>Interval at which system maintenance is run. 43200 (12 hours) is often good enough.</td>
</tr>
<tr>
    <td>Recipients</td>
    <td>array of strings</td>
    <td>These Email addresses will receive the maintenance summary report.</td>
</tr>
<tr>
    <td>TCPPorts</td>
    <td>array of integers</td>
    <td>Check the status of these TCP ports during maintenance. It is useful to have all daemon ports (such as HTTP 80, mail 25) here.</td>
</tr>
</table>

Here is an example configuration that also checks the port status of mail(25), DNS(53), and HTTP(80, 443) daemons:
<pre>
{
    ...

    "Maintenance": {
        "IntervalSec": 43200,
        "Recipients": ["howard@gmail.com"],
        "TCPPorts": [
            25,
            53,
            80,
            443
        ]
    },

    ...
}
</pre>

## Run
Tell laitos to run periodic system maintenance in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,maintenance,...

## Usage
System maintenance is run automatically 10 minutes after laitos daemon starts up, and then at regular interval
(specified in configuration).

Manual action is not required.

## Tips
System maintenance does not have to run too often. Running at interval of 12 hours (432000 seconds) or 24 hours
(864000 seconds) is often good enough.

The additional softwares installed during maintenance are:
- Dependencies of PhantomJS, used by [browser-in-browser](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-browser-in-browser)
  and [interactive web browser](https://github.com/HouzuoGuo/laitos/wiki/Toolbox:-interactive-web-browser). The two
  features may not function properly until system maintenance has run for the first time.
- Utilities for synchronising system clock.
- Zip file tools that are useful for maintaining application bundles for [public cloud](https://github.com/HouzuoGuo/laitos/wiki/Public-cloud).
- Network diagnosis tools.