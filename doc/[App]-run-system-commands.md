# Introduction

Via any of enabled laitos daemons, you may run operating system (shell)
commands. This works on all OS platforms supported by laitos.

# Configuration

This app is always available for use and does not require configuration.

However, if you wish to override the automatic detection of system shell
interpreter, then navigate to JSON object `Features` and construct a JSON object
called `Shell` with the following properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>InterpreterPath</td>
    <td>string</td>
    <td>
        Absolute path to shell interpreter program, such as /bin/bash
        <br/>
        If left empty, laitos will automatically discover a shell interpreter.
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
            "InterpreterPath": "/bin/bash"
        },
        ...
    },

    ...
}
</pre>


# Usage

Use any capable laitos daemon to invoke the app:

    .s <shell command to run>

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
- When shell commands are run, the environment variable `PATH` is hard coded to
  `/tmp/laitos-util:/bin:/sbin:/usr/bin:/usr/sbin:/usr/libexec:/usr/local/bin:/usr/local/sbin:/opt/bin:/opt/sbin`
- `/tmp/laitos-util` is automatically maintained by laitos internally to store
  non-essential executables such as a copy of busybox and a copy of toybox.
