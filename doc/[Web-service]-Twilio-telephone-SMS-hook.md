## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the web service is triggered
by incoming calls and SMS from Twilio platform, and let caller/sender invoke app commands.

Which means - your personal Emails, twitter, and many other Internet features become accessible over telephone, SMS text, and
even satellite terminals!

## Preparation
1. Sign up for an account at [twilio.com](https://www.twilio.com) - an API platform that connects computer programs with
   public switched telephone and mobile network. Sign up is free.
2. Visit Twilio developer's console, then [purchase a phone number](https://www.twilio.com/console/phone-numbers/search).
   Make sure the number can make calls and SMS - not all numbers come with these capabilities! A number costs between
   2-10 USD/month to own, and each call/SMS costs extra.

If you have or plan to configure the app that makes [outgoing calls and SMS](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-making-calls-and-send-SMS),
feel free to use identical Twilio account and phone number configuration in this web service.

## Configuration
Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
JSON key `HTTPFilters`. Make sure to limit `MaxLength` of `LintText` to a reasonable number below 1000, otherwise an
unexpectedly large command response may incur high fees.

Then, in order to enable telephone call hook, construct the following properties under JSON key `HTTPHandlers`:
1. A string property called `TwilioCallEndpoint`, value being the URL location that will serve the form. Keep the
   location a secret to yourself and make it difficult to guess.
2. An object called `TwilioCallEndpointConfig` with only a string property `CallGreeting`, value being a greeting
   message spoken to telephone caller.

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "TwilioCallEndpoint": "/very-secret-twilio-call-service",
        "TwilioCallEndpointConfig": {
            "CallGreeting": "Hello from laitos"
        },
        "TwilioSMSEndpoint": "/very-secret-twilio-sms-service",

        ...
    },

    ...

    "HTTPFilters": {
        "PINAndShortcuts": {
            "Passwords": ["verysecretpassword"],
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
            "CompressSpaces": true,
            "CompressToSingleLine": true,
            "KeepVisible7BitCharOnly": true,
            "MaxLength": 160,
            "TrimSpaces": true
        },
        "NotifyViaEmail": {
            "Recipients": ["me@example.com"]
        }
    },
    ...
}
</pre>

## Run
The service is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
1. Visit [phone numbers management](https://www.twilio.com/console/phone-numbers/incoming) and click on the freshly
   purchased number to enter its configuration page.
2. To let laitos handle telephone calls: enter the following configuration under "Voice & Fax" section:

       Accept incoming: Voice Calls
       Configure with:  Webhooks, or TwiML Bins or Functions
       A call comes in: Webhook, HTTP POST, and enter laitos server address `TwilioCallEndpoint`

   For an example, the laitos server address may be `https://my-laitos-server.com/very-secret-twilio-call-service`

3. To let laitos handle SMS messages: enter the following configuration under "Messaging" section:

       Configure With:     Webhooks, or TwiML Bins or Functions
       A message comes in: Webhook, HTTP POST, and enter laitos server address `TwilioSMSEndpoint`

   For an example, the laitos server address may be `https://my-laitos-server.com/very-secret-twilio-sms-service`

Then, in an SMS, enter password and app command and send the text to your Twilio phone number. Wait several
seconds and the command result will arrive in an SMS reply.

In order to enter app command via telephone call, use the number pad to dial password and app command, completed with a
pound '#' sign, then wait for command execution and then a spoken response. The number pad input works in this way:
- The number pad is able to enter nearly all Latin letters, common symbols, and numbers.
- A character is entered via either a single digit or a sequence of digits.
- Asterisk toggles between upper case and lower case letters. By default letters are in lower case.
- Digit 0 either terminates a character's sequence, or generate spaces if character's sequence is already terminated.
- A new character sequence begins automatically if previous character sequence is terminated or this number does not
  continue the number sequence (e.g. sequence "3334" generates an "f" letter and then awaits more input after "4").
- Symbols and numbers always require explicit termination of their sequence by a digit 0.

Here are the digit sequences for entering letters, symbols, and numbers:
<pre>
111 - !  112 - @  113 - #  114 - $  115 - %  116 - ^  117 - &  118 - *  119 - (  121 - backtick
122 - ~  123 - )  124 - -  125 - _  126 - =  127 - +  128 - [  129 - {  131 - ]  132 - }
133 - \  134 - |  135 - ;  136 - :  137 - '  138 - "  139 - ,  141 - <  142 - .  143 - >
144 - /  145 - ?  0 – Space

1 – 0  11 – 1  12 – 2  13 – 3  14 – 4  15 – 5  16 – 6  17 – 7  18 – 8  19 - 9

2 - a      22 - b     222 – c    3 - d      33 - e     333 - f
4 - g      44 - h     444 – I    5 - j      55 - k     555 - l
6 - m      66 - n     666 – o    7 - p      77 - q     777 - r    7777 - s
8 - t      88 - u     888 – v    9 - w      99 - x     999 - y    9999 – z
</pre>

If you wish the output to be spelt phonetically rather than spoken, input number sequence `0123` before and command
input. This technique is very useful for copying sophisticated command output such as those from operating system shell
commands.

## Tips
Telephone and mobile networks are prone to eavesdropping attacks that can reveal your password and app command responses
to potential attackers. Consider using [one-time password in place of password](https://github.com/HouzuoGuo/laitos/wiki/Command-processor#use-one-time-password-in-place-of-password).

The web service does not respond if an SMS sender fails to use the correct password. All SMS and calls are logged for
inspection on Twilio console.

Regarding laitos configuration:
- Make sure to choose a very secure URL for both call and SMS endpoints, it is the only way to secure this web service!
- Under `HTTPFilters`, double check that `MaxLength` of `LintText` is set to a reasonable number below 1000, otherwise
  if laitos sends an exceedingly large SMS response, Twilio will break apart the response into multiple SMS segments,
  and then charge a high fee for sending the segments altogether! Also, consider turning on all compression features of
  `LintText` to further reduce cost.
- To prevent spam, laitos limits number of incoming calls to once every 10 seconds per each caller, and limits incoming SMS
  messages to once every 10 seconds per each sender.

Regarding Twilio configuration:
- Usage of HTTPS is mandatory in web hook, your laitos web server must be serving HTTPS traffic using a valid TLS
  certificate chain.
- If you run identical laitos configuration on more than one servers for fail-over, then you may enter the secondary
  server's web hook address under Twilio configuration's "Primary Handler Fails" input. Twilio will then automatically
  uses the secondary server if primary server fails.
- It is OK to bind more than one Twilio phone numbers to the same laitos server that offers this web service.
