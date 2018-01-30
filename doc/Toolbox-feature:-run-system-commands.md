# Toolbox feature: run system commands

## Introduction
Via any of enabled laitos daemons, you may run operating system (shell) commands and read the response.

## Configuration
Configuration is generally not needed for this feature.

However, if you wish to override the automatic choice of shell interpreter:

Under JSON object `Features`, construct a JSON object called `Shell` that has the following properties:
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
        If left empty, laitos will automatically find a working shell program already installed on the computer.
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


## Usage
Use any capable laitos daemon to run the following toolbox command:

    .s <shell command to run>

The shell command may use shell interpreter's capabilities to their full extend. For example, the following command will
find system users whose name contains "howard" and store them in an output file:

    .s cat /etc/passwd | grep howard > output.txt

## Tips
- Shell commands will run on most Unix-like operating systems such as Linux, BSD, and MacOS. This feature does not yet
  support Windows.
- If `InterpreterPath` is not specified, laitos will look for shell interpreter among `/bin`, `/usr/bin`,
  `/usr/local/bin`, `/opt/bin`, in this order: `bash`, `dash`, `zsh`, `ksh`, `ash`, `tcsh`, `csh`, `sh`.
- When shell commands are run, the environment variable `PATH` is hard coded to
  `/tmp/laitos-util:/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:/opt/bin:/opt/sbin`
- `/tmp/laitos-util` is maintained by laitos internally to store non-essential components, such as a copy of PhantomJS
  software, a copy of BusyBox software, and a copy of ToyBox software. If the components are available in laitos current
  working directory when it starts up, the components will be copied into `/tmp/laitos-util`.