## Introduction
The mail server forwards arriving mails as-is to your personal mail address. No mails are stored on the server after
they are forwarded.

With additional configuration, the server will execute password-protected app commands from incoming mail, and mail
command response back to the sender.

For communication secrecy, the server supports StartTLS operation and identifies itself with a TLS certificate.

## Preparation
In order for an Internet user to successfully send mails to your domain names, they must be covered by a DNS hosting
service. If the concept sounds unfamiliar, check out this article from Amazon Web Service: [What is DNS](https://aws.amazon.com/route53/what-is-dns/).

DNS hosting providers usually charge ~ 1 USD per domain name per month. If you are looking for a provider, check out:
- [Amazon Web Service "Route53"](https://aws.amazon.com/route53/)
- [Google Cloud Platform "Cloud DNS"](https://cloud.google.com/dns/)

After signing up for DNS hosting service, they will give you a set of NS addresses (usually four) for each domain. Then
you need to let Domain Registrar know by giving the NS addresses to each domain name's configuration; it takes up to 24
hours for this change to propagate through the Internet.

The [laitos DNS server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server) is a DNS relay, it is _not_ a DNS
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
        "ForwardTo": ["me@example.com", "me2@example.com"],
        "MyDomains": ["my-home.example.com", "my-blog.example.com"],

        "TLSCertPath": "/root/example.com.crt",
        "TLSKeyPath": "/root/example.com.key"
    },

    ...
}
</pre>

## App command processor
In order for mail server to invoke app commands from mail content, complete all of the following:

1. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `MailFilters`.
2. Follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration) to
   construct configuration for sending mail replies.

Here is an example:
<pre>
{
    ...

    "MailDaemon": {
        "ForwardTo": ["me@example.com", "me2@example.com"],
        "MyDomains": ["my-home.example.com", "my-blog.example.com"],

        "TLSCertPath": "/root/example.com.crt",
        "TLSKeyPath": "/root/example.com.key"
    },

    "MailFilters": {
        "PINAndShortcuts": {
            "Passwords": ["VerySecretPassword"],
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
Tell laitos to run mail daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,smtpd,...

## Deployment
At your DNS hosting provider, create or modify a DNS "MX" entry for each of `MyDomains`. The entry must look like:

- DNS name: `my-domain-name.net`
- Record type: `MX`
- Time to live (TTL): leave at default or `5 minutes`
- Value (preference and mail server): `10 laitos-server-public-IP`

Here are couple of examples involving, assuming that laitos server is on `123.234.123.234`:

<table>
<tr>
    <th>DNS name</th>
    <th>Record type</th>
    <th>Time to live (TTL)</th>
    <th>Value</th>
    <th>Remark</th>
</tr>
<tr>
    <td>my-domain-name.net</td>
    <td>MX</td>
    <td>5 minutes</td>
    <td>10 123.234.123.234</td>
    <td>Domain name example</td>
</tr>
<tr>
    <td>my-home.example.com</td>
    <td>MX</td>
    <td>5 minutes</td>
    <td>10 123.234.123.234</td>
    <td>Sub-domain example</td>
</tr>
<tr>
    <td>my-blog.example.com</td>
    <td>MX</td>
    <td>5 minutes</td>
    <td>10 123.234.123.234</td>
    <td>Another sub-domain example</td>
</tr>
</table>

Wait up to an hour for new DNS records to propagate through the Internet.

## Test
Send a test mail with subject, text, and attachments to any name under `MyDomains` (e.g. `i@my-domain-name.net`). Wait
a short moment, check the inbox on any of `ForwardTo` address (e.g. `me@example.com`), the test mail should arrive at
all of the `ForwardTo` addresses.

Try invoking an app command - send laitos server a mail with arbitrary subject, and write down password PIN and app command
in the content body. Look for the command response in a mail replied to the sender.

## Tips
- Mail servers are often targeted by spam mails - but don't worry, use a personal mail service that comes with strong
  spam filter (such as Gmail) as `ForwardTo` address, then spam mails will not bother you any longer.
- Occasionally spam filter (such as Gmail's) may consider legitimate mails forwarded by laitos as spam, therefore please
  check your spam folders regularly.
- Many Internet domain names use [DMARC](https://en.wikipedia.org/wiki/DMARC) to protect their business from mail spoofing.
  Though laitos usually forwards the verbatim copy of incoming mail to you, DMARC makes an exception - laitos has to change
  the sender from `name@protected-domain.com` to `name@protected-domain-laitos-nodmarc-###.com` where hash is a random digit.
  Otherwise your mail provder will discard the mail silently - without a trace in spam folder.
