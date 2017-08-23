# Daemon: plain text sockets

## Introduction
The plain text sockets provide access to toolbox features via very basic client programs, such as `telnet`, `netcat`,
and `HyperTerminal`.

The sockets are served via both TCP and UDP ports in plain text.

Due to the incredibly simple communication protocol, the text information exchanged between server and client are prone
to attacks such as eavesdropping.

## Configuration
1. Construct the following JSON object and place it under JSON key `TelegramBot` in configuration file.
   The following properties are mandatory:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The address network to listen to. It is usually "0.0.0.0", which means listen on all network interfaces.</td>
</tr>
<tr>
    <td>TCPPort</td>
    <td>integer</td>
    <td>TCP port number to listen to. Use 0 to disable the TCP listener.</td>
</tr>
<tr>
    <td>UDPPort</td>
    <td>integer</td>
    <td>UDP port number to listen on. Use 0 to disable the UDP listener.</td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>How many times in ten-second interval a client (identified by IP) is allowed to communicate with the server.</td>
</tr>
</table>

2. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `PlainTextBridges`.

Here is an example setup of mail server with command processor:
<pre>
{
    ...
    
    "PlainTextDaemon": {
        "Address": "0.0.0.0",
        "TCPPort": 53,
        "UDPPort": 53,
        "PerIPLimit": 10
    },
    "PlainTextBridges": {
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

    sudo ./laitos -config <CONFIG FILE> -frontend ...,plaintext,...

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
The plain text daemon ensures that toolbox features remain accessible via basic tools and unencrypted communication, in
the unlikely event of losing access to all other daemons. The unencrypted nature of communication opens up possibility
of eavesdropping, therefore you should use the plain socket daemon only as a last resort.

UDP is designed to be a less reliable protocol capable of carrying less data, be aware that some home/work networks
deliberately block outgoing UDP traffic.