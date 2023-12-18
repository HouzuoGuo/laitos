# Introduction

Via any of enabled laitos daemons, you may run operating system (shell)
commands. This works on all OS platforms supported by laitos.

# Configuration

This app is always enabled for use in the default configuration, which restricts
command execution to these programs without additional parameters:

- arch arp blkid cal date df dmesg dnsdomainname false free
- groups hostid hostname id ifconfig ipconfig iostat ipcs
- kbd_mode ls lsof lspci lsusb mpstat netstat nproc
- ps pstree pwd route stty tty uname uptime whoami

To customise configuration, under JSON object `Features`, construct a JSON
object called `Shell` that has the following mandatory properties:

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>InterpreterPath</td>
    <td>string</td>
    <td>
        Absolute path to the system shell interpreter, such as /bin/bash.
    </td>
    <td>
        Automatically detected, e.g. bash, powershell.
    </td>
</tr>
<tr>
    <td>Unrestricted</td>
    <td>true/false</td>
    <td>
        When true, allow executing arbitrary commands incl. CLI parameters.
    <td>
        False - only allow executing predefined &amp; hard-coded commands.
    </td>
</tr>
</table>

Here is an example:

<pre>
{
    ...

    "Features": {
        ...

        "Shell": {
            "InterpreterPath": "/bin/bash",
            "Unrestricted": true
        },
        ...
    },

    ...
}
</pre>

# Usage

Use any capable laitos daemon to invoke the app:

    .s shell-command

The shell command may use shell interpreter's capabilities to their full extend.
For example, the following command will find system users whose name contains
"howard" and store them in an output file:

    .s cat /etc/passwd | grep howard > output.txt

# Tips

- When `InterpreterPath` is left empty, laitos will automatically look for a
  shell interpreter from `/bin`, `/usr/bin`, `/usr/local/bin`, `/opt/bin`,
  laitos recognises `bash`, `dash`, `zsh`, `ksh`, `ash`, `tcsh`, `csh`, `sh`.
- On Windows, laitos uses PowerShell by default, unless an alternative shell
  interpreter is specified in configuration.
- On Linux, the `PATH` is hard-coded to
  `/tmp/laitos-util:/bin:/sbin:/usr/bin:/usr/sbin:/usr/libexec:/usr/local/bin:/usr/local/sbin:/opt/bin:/opt/sbin`
  when executing shell commands.
- laitos automatically copies some non-essential executables such as busybox and
  toybox into `/tmp/laitos-util`.
