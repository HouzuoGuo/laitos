# Daemon: mail server

## Introduction
The mail server forwards arriving mails as-is to your personal Email address. No mails are stored on the server after
they are forwarded.

With additional configuration, the server will process toolbox feature command from incoming mail, and mail response to
the sender.

For communication secrecy, the server supports StartTLS operation and identifies itself with TLS certificate.

## Preparation
In order for an Internet user to successfully send mails to your domain names, they must be covered by a DNS hosting
service. If the concept sounds unfamiliar, check out this article from Amazon Web Service: [What is DNS](https://aws.amazon.com/route53/what-is-dns/).

DNS hosting providers usually charge ~ 1 USD per domain name per month. If you are looking for a provider, check out:
- [Amazon Web Service "Route53"](https://aws.amazon.com/route53/)
- [Google Cloud Platform "Cloud DNS"](https://cloud.google.com/dns/)

After signing up for DNS hosting service, they will give you a set of NS addresses (usually four) for each domain. Then
you need to let Domain Registrar know by giving the NS addresses to each domain name's configuration; it takes up to 24
hours for this change to propagate through the Internet.

The [laitos DNS server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-DNS-server) is a DNS relay, it is _not_ a DNS
hosting service.

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
    <td>Port number to listen on. It is usually 25 - the port number designated for SMTP.</td>
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

The following properties are optional under JSON key `MailDaemon`:

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
In order to let mail server process toolbox feature commands from mail body, complete all of the following:

1. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `MailFilters`.
2. Follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration) to construct
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
    
    "MailFilters": {
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
Tell laitos to run mail daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -frontend ...,smtpd,...

## Deployment
At your DNS hosting provider, create or modify a DNS "MX" entry for each of `MyDomains`. The entry must look like:

- DNS name: `my-domain-name.net`
- Record type: `MX`
- Time to live (TTL): leave at default or `5 minutes`
- Value (preference and mail server): `10 laitos-server-public-IP`

Here is an example involving two domain names and three MX entries, assuming that laitos server is on `58.169.236.112`:

<table>
<tr>
    <th>DNS name</th>
    <th>Record type</th>
    <th>Time to live (TTL)</th>
    <th>Value</th>
    <th>Remark</th>
</tr>
<tr>
    <td>howard-homepage.net</td>
    <td>MX</td>
    <td>5 minutes</td>
    <td>10 58.169.236.112</td>
    <td>First example</td>
</tr>
<tr>
    <td>howard-blog.org</td>
    <td>MX</td>
    <td>5 minutes</td>
    <td>10 58.169.236.112</td>
    <td>Second example</td>
</tr>
<tr>
    <td>cool.howard-blog.org</td>
    <td>MX</td>
    <td>5 minutes</td>
    <td>10 58.169.236.112</td>
    <td>A sub-domain of second example</td>
</tr>
</table>

Wait up to an hour for new DNS records to propagate through the Internet.

## Test
Send a test mail with subject, text, and attachments to any name under `MyDomains` (e.g. `i@howard-blog.org`). Wait a
short moment, check the inbox on any of `ForwardTo` address (e.g. `howard@gmail.com`), the test mail should arrive at
all of the `ForwardTo` addresses.

To try the toolbox command processor, send a mail with any subject, and write down toolbox command in the mail body.
Send it out, wait a short moment, and check the sender's inbox for command response.

Don't forget to put password PIN in front of the toolbox command!

## Tips
Mail servers are often targeted by spam mails. But don't worry, use a personal mail service that comes with strong spam
filter (such as Gmail) as `ForwardTo` address, and spam mails will not bother you any longer.