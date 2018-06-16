# Daemon: mail server

## Introduction
The mail server forwards arriving mails as-is to your personal mail address. No mails are stored on the server after
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
Construct the following JSON object and place it under JSON key `MailDaemon` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>MyDomains</td>
    <td>array of strings</td>
    <td>
        Domain names to receive mails for.
        <br/>
        Example: ["my-blog.net", "my-homepage.org"].
    </td>
    <td>(This is a mandatory property without a default value)</td>
</tr>
<tr>
    <td>ForwardTo</td>
    <td>array of strings</td>
    <td>
        Forward incoming mails to these addresses.
        <br/>
        Example: ["me@gmail.com", "me@hotmail.com"].
    </td>
    <td>(This is a mandatory property without a default value)</td>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The address network to listen on.</td>
    <td>"0.0.0.0" - listen on all network interfaces.</td>
</tr>
<tr>
    <td>Port</td>
    <td>integer</td>
    <td>UDP port number to listen on.</td>
    <td>25 - the well-known port number designated for mail service (SMTP).</td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>Maximum number of mails a client (identified by IP) may deliver to this server in a second.</td>
    <td>4 - good enough to prevent flood of spam</td>
</tr>
<tr>
    <td>TLSCertPath</td>
    <td>string</td>
    <td>
        Absolute or relative path to PEM-encoded TLS certificate file.
        <br/>
        The file may contain a certificate chain with server certificate on top and CA authority toward bottom.
    </td>
    <td>(Not enabled by default)</td>
</tr>
<tr>
    <td>TLSKeyPath</td>
    <td>string</td>
    <td>Absolute or relative path to PEM-encoded TLS certificate key.</td>
    <td>(Not enabled by default)</td>
</tr>
</table>


Here is a minimal setup example that enables TLS as well:
<pre>
{
    ...

    "MailDaemon": {
        "ForwardTo": ["howard@gmail.com", "howard@hotmail.com"],
        "MyDomains": ["howard-homepage.net", "howard-blog.org"],

        "TLSCertPath": "/root/howard-blog.org.crt",
        "TLSKeyPath": "/root/howard-blog.org.key"
    },

    ...
}
</pre>

## Toolbox command processor
In order for mail server to process toolbox feature commands from mail content, complete all of the following:

1. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `MailFilters`.
2. Follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration) to
   construct configuration for sending mail replies.

Here is a minimal setup example that comes enables command processor:
<pre>
{
    ...

    "MailDaemon": {
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

    sudo ./laitos -config <CONFIG FILE> -daemons ...,smtpd,...

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
- Mail servers are often targeted by spam mails - but don't worry, use a personal mail service that comes with strong
  spam filter (such as Gmail) as `ForwardTo` address, then spam mails will not bother you any longer.
- Occasionally spam filter (such as Gmail's) may consider legitimate mails forwarded by laitos as spam, therefore please
  check your spam folders regularly.