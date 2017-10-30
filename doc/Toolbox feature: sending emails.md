# Toolbox feature: sending emails

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

    .m recipient@example.net "this is email subject" this is email body

laitos will then reply to the command with number of characters of the email body content, or `sending in background` if
it takes longer than usual to deliver the email.

Be aware that email subject is surrounded by double quotes, therefore the subject itself may not contain double quote.

## Send SOS email
Warning! Do not send SOS emails unless you are in life-threatening danger. laitos developer Houzuo (Howard) Guo does not
guarantee that SOS emails will be successfully delivered to any search-and-rescue institution; the developer cannot be
held responsible for monetary and legal consequences associated with the SOS emails delivered under genuine danger or
accidental trigger.

Use any capable laitos daemon to send an SOS email, note the special recipient `sos@sos`:

    .m sos@sos "this is my subject" this is my email body

laitos will then reply to the command `Sending SOS`, and SOS emails will be sent in background to all of the following
search-and-rescue institutions worldwide:
- Australia MSAR/ASAR
- Canada JRCC
- Greece JRCC
- Finland MRCC
- Hong Kong ARCC and MRCC
- Israel MRCC
- Japan MCC
- P.R.China MCC and MRCC
- Russia MRCC
- United Kingdom ARCC and MCC

The SOS email looks like:

    Subject: SOS HELP <and your subject>
    Body:
    SOS HELP!
    The time is <UTC time of laitos server>.
    This is the operator of IP address <public IP address of laitos server>.
    Please send help: <and your Email body content>.