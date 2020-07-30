## Introduction
The plain text telnet server provide access to app commands via rudimentary client software such as `telnet`, `netcat`,
and `HyperTerminal`.

The server is capable of operating on both TCP and UDP. Due to the simplicity of underlying protocol, data are exchanged
in plain text between client software and this server daemon. See tips below.

## Configuration
1. Construct the following JSON object and place it under JSON key `PlainSocketDaemon` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>TCPPort</td>
    <td>integer</td>
    <td>TCP port number to listen to. Use 0 to disable the TCP listener.</td>
    <td>(This is a mandatory property without a default value)</td>
</tr>
<tr>
    <td>UDPPort</td>
    <td>integer</td>
    <td>UDP port number to listen on. Use 0 to disable the UDP listener.</td>
    <td>(This is a mandatory property without a default value)</td>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The address network to listen on.</td>
    <td>"0.0.0.0" - listen on all network interfaces.</td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>Maximum number of times a client (identified by IP) may communicate with the server in a second.</td>
    <td>2 - good enough for personal use</td>
</tr>
</table>

2. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `PlainSocketFilters`.

Here is a minimal setup example:
<pre>
{
    ...

    "PlainSocketDaemon": {
        "TCPPort": 53,
        "UDPPort": 53
    },
    "PlainSocketFilters": {
        "PINAndShortcuts": {
            "PIN": "VerySecretPassword",
            "Shortcuts": {
                "watsup": ".eruntime",
                "EmergencyStop": ".estop",
                "EmergencyLock": ".elock"
            }
        },
        "TranslateSequences": {
            "Sequences": [
                ["#/", "|"]
            ]
        },
        "LintText": {
            "CompressSpaces": false,
            "CompressToSingleLine": false,
            "KeepVisible7BitCharOnly": false,
            "MaxLength": 4096,
            "TrimSpaces": false
        },
        "NotifyViaEmail": {
            "Recipients": ["me@example.com"]
        }
    },

    ...
}
</pre>

## Run
Tell laitos to run chat bot daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,plainsocket,...

## Usage
Use any TCP/UDP connection tool such as `netcat` (also called `nc`), `telnet` (TCP only), or `HyperTerminal` (TCP only)
to connect to plain text daemon:

    nc <laitos-server-IP> <TCPPort>

And type an app command (the example uses a system shell command to retrieve system uptime):

    VerySecretPassword .s uptime
    11:09am  up   2:58,  3 users,  load average: 0.23, 0.29, 0.27 (the response)

The UDP socket functions in similar way (the `-u` switch tells `netcat` to make UDP connection):

    nc -u <laitos-server-IP> <UDPPort>

And type app commands similar to the TCP example.

## Tips
- The plain text daemon helps to invoke app commands in the unlikely event of losing access to all other daemons.
  The primitive nature of the protocol opens up possibility of eavesdropping, consider using [one-time password in place of password PIN](https://github.com/HouzuoGuo/laitos/wiki/Command-processor#use-one-time-password-in-place-of-password-pin).
- The size of app command and command response are limited to roughly 1200 characters each when using UDP.
- When using Telnet client program on Linux and Unix, the client program displays (echos) the keyboard input automatically.
  However on Windows the Telnet program does not display input automatically, to work around it, type `Ctrl+]` after establishing
  telnet connection, and then type `set localecho` to enable input display.
