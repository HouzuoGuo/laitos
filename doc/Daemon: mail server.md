# Daemon: mail server

## Introduction
The mail server forwards arriving mails as-is to your personal Email address. No mails are stored on the server after
they are forwarded.

With additional configuration, the server will process toolbox feature command from incoming mail, and mail response to
the sender.

For communication secrecy, the server supports StartTLS operation and identifies itself with TLS certificate.

## Configuration
Construct the following JSON object and place it under JSON key `MailDaemon` in configuration file. The following
properties are mandatory:
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
    <td>Port</td>
    <td>integer</td>
    <td>TCP port number to listen on. It is usually 25 - the port number designated for SMTP.</td>
</tr>
<tr>
    <td>MyDomains</td>
    <td>array of strings</td>
    <td>
        Domain names to receive emails for.
        <br/>
        Example: ["my-blog.net", "my-homepage.org"].
    </td>
</tr>
<tr>
    <td>ForwardTo</td>
    <td>array of strings</td>
    <td>
        Forward incoming mails to these Email addresses.
        <br/>
        Example: ["me@gmail.com", "me@hotmail.com"].
    </td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>
        How many times in ten-second interval server will accept mails from a client (identified by IP).
        <br/>
        3 is usually enough.
    </td>
</tr>
</table>

The following properties are optional:

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>TLSCertPath</td>
    <td>string</td>
    <td>
        Absolute or relative path to PEM-encoded TLS certificate file.
        <br/>
        The file may contain a certificate chain with server certificate on top and CA authority toward bottom.
    </td>
</tr>
<tr>
    <td>TLSKeyPath</td>
    <td>string</td>
    <td>Absolute or relative path to PEM-encoded TLS certificate key.</td>
</tr>
</table>


Remember to tell laitos to run DNS daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -frontend ...,smtpd,...

Here is an example setup made for two imaginary domain names:
<pre>
{
    ...
    
    "MailDaemon": {
        "Address": "0.0.0.0",
        "Port": 25
        "PerIPLimit": 3,
        
        "ForwardTo": ["howard@gmail.com", "howard@hotmail.com"],
        "MyDomains": ["howard-homepage.net", "howard-blog.org"],
        
        "TLSCertPath": "/root/howard-blog.org.crt",
        "TLSKeyPath": "/root/howard-blog.org.key"
    },
     
    ...
}
</pre>

## Toolbox command processor
In order to let mail server process toolbox feature commands, complete all of the following:

1. Construct the following JSON object and place it under JSON key `MailProcessor` in configuration file.
   The following properties are mandatory:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>CommandTimeoutSec</td>
    <td>integer</td>
    <td>Toolbox command aborts after this many seconds go by. 120 is usually good enough.</td>
</tr>
</table>

2. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `MailBridges`.
3. Follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration) to construct
   configuration for sending Email responses.

Here is an example setup of mail server with command processor:
<pre>
{
    ...
    
    "MailDaemon": {
        "Address": "0.0.0.0",
        "Port": 25
        "PerIPLimit": 3,
        
        "ForwardTo": ["howard@gmail.com", "howard@hotmail.com"],
        "MyDomains": ["howard-homepage.net", "howard-blog.org"],
        
        "TLSCertPath": "/root/howard-blog.org.crt",
        "TLSKeyPath": "/root/howard-blog.org.key"
    },
    
    "MailBridges": {
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

## Test

## Usage
