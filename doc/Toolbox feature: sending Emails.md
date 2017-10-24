# Toolbox feature: sending Emails

## Introduction
Via any of enabled laitos daemons, you may send Emails to friends and anyone online.

## Configuration
Be aware that the configuration below is entirely independent from the shared
[outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration).
However, feel free to use identical configuration for both this feature and the shared configuration.

Under JSON object `Features`, construct a JSON object called `SendMail` that has an object called `MailCient`
with the following mandatory properties:

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

Here is an example for using [SendGrid](https://sendgrid.com/) to send outgoing Emails:
<pre>
{
    ...

    "Features": {
        ...

        "SendMail": {
            "MailClient": {
                "AuthPassword": "SG.aabbccddeeffgghhiijjkkllmmnnooppqqrrssttuuvvwwxxyyzz",
                "AuthUsername": "apikey",
                "MTAHost": "smtp.sendgrid.net",
                "MTAPort": 2525,
                "MailFrom": "i@howard.gg"
            }
        },
        
        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to send an email:

    .m to@example.net "this is a subject" this is mail body

Be aware that Email subject is surrounded by double quotes, therefore the subject itself may not contain double quote.