# Outgoing mail configuration

## Introduction
A shared outgoing mail configuration must be created, in order for the following components to send emails:
- Daemon: [system maintenance](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-system-maintenance)
- Daemon: [mail server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-mail-server)
- Web service: [GitLab browser](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-GitLab-browser)
- `NotifyViaEmail` of [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) together with daemons that embed command processor.
- Toolbox feature: [sending emails](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-sending-emails).

## Configuration
Construct the following object under JSON key `MailClient`, all properties are mandatory:

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>AuthUsername</td>
    <td>string</td>
    <td>SMTP access username. Leave it empty if not required.</td>
</tr>
<tr>
    <td>AuthPassword</td>
    <td>string</td>
    <td>SMTP access password. Leave it empty if not required.</td>
</tr>
<tr>
    <td>MTAHost</td>
    <td>string</td>
    <td>Host name of MTA on SMTP server, for example "smtp.sendgrid.net".</td>
</tr>
<tr>
    <td>AuthPassword</td>
    <td>integer</td>
    <td>Port of MTA on SMTP server, for example 2525.</td>
</tr>
<tr>
    <td>MailFrom</td>
    <td>string</td>
    <td>"From" address to appear in outgoing mails.</td>
</tr>
</table>


## Configuration example
Here is an example for using [SendGrid](https://sendgrid.com/) to send outgoing emails:
<pre>
{
    ...
    
    "MailClient": {
        "AuthPassword": "SG.aabbccddeeffgghhiijjkkllmmnnooppqqrrssttuuvvwwxxyyzz",
        "AuthUsername": "apikey",
        "MTAHost": "smtp.sendgrid.net",
        "MTAPort": 2525,
        "MailFrom": "i@howard.gg"
    },
    
    ...
}
</pre>

## Tips
If laitos is running on public cloud, be aware that several public cloud providers (such as Google Compute Engine) does
not allow servers themselves to deliver any email via local mail transportation agents (e.g. postfix, sendmail).
This is usually done to prevent spam.

In that case, you will have to use a mail delivery service such as [SendGrid](https://sendgrid.com/) to send emails.
For the case of Google Compute Engine, check out this detailed topic written by Google:
[Sending Email from an Instance](https://cloud.google.com/compute/docs/tutorials/sending-mail/)