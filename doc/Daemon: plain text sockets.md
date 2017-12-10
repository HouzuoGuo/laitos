# Daemon: plain text sockets

## Introduction
The plain text sockets provide access to toolbox features via very basic client programs, such as `telnet`, `netcat`,
and `HyperTerminal`.

The sockets are served via both TCP and UDP ports in plain text.

Due to the incredibly simple communication protocol, the text information exchanged between server and client are prone
to attacks such as eavesdropping, therefore only use plain text sockets in trusted private network!

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
                "ILoveYou": ".eruntime",
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
            "Recipients": ["howard@gmail.com"]
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

And issue a toolbox command (the example asks how long the computer has been running):

    VerySecretPassword .s uptime
    11:09am  up   2:58,  3 users,  load average: 0.23, 0.29, 0.27 (the response)

The UDP socket functions in similar way (the `-u` switch tells `netcat` to make UDP connection):

    nc -u <laitos-server-IP> <UDPPort>

And issue toolbox command just like the TCP example.

## Tips
- The plain text daemon ensures that toolbox features remain accessible via basic tools and unencrypted communication,
  in the unlikely event of losing access to all other daemons. The unencrypted nature of communication opens up possibility
  of eavesdropping, therefore you should use the plain socket daemon only as a last resort.
- UDP is less reliable protocol and can only carry a small amount of data. Be aware that some home/work networks
  deliberately block UDP traffic.