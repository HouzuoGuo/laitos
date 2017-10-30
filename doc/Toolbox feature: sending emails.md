# Toolbox feature: sending emails

## Introduction
Via any of enabled laitos daemons, you may send Emails to friends and anyone online.

## Configuration
This toolbox feature uses the common [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration).

Complete the common outgoing mail configuration, and this feature will be automatically enabled.

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