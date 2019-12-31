# Command processor

## Introduction
The following daemon components have an embedded command processor to let users invoke app commands:
- [DNS server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server)
- [Mail server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-mail-server)
- [Serial port communicator](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-serial-port-communicator)
- [Telnet server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telnet-server)
- [Telegram chat-bot](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telegram-chat-bot)
- Web service [invoke app command](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-invoke-app-command)
- Web service [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook)
- Web service [Microsoft bot hook](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Microsoft-bot-hook)

During app command invocation, the following actions take place:
1. The user enters a command, for example, by using the "invoke app command" web service form, or by sending an app command
   in an Email addressed to laitos mail server. (e.g. `mypass .e info`)
2. laitos validates the password PIN from the input command to match configuration from `PINAndShortcuts`, or if the input
   is a shortcut, expands the shortcut into full command without looking for a password PIN.
3. laitos walks the app command (not the password PIN) through `TranslateSequences` mechanism that replaces sequence of
   characters by a different sequence.
4. laitos identifies the app (e.g. `.e` for program control) and gives the app remainder of the command input for parameters.
5. The app routine runs and produces plain text response.
6. laitos walks the text response through `LintText` mechanism that compacts and tidies up the text if needed. As a special
   case, if the app produces an empty response, the actual app response will change to `EMPTY OUTPUT`.
7. laitos informs the user about the app response via on-screen display, message reply, or other means.
8. In background, laitos sends notification Emails with the app command and text response to a list of optional recipients.

## Configuration
Construct the following objects under JSON key (e.g. `HTTPFilters`, `MailFilters`) named by individual daemon - you may
find them in daemon's usage manual.

Mandatory `PINAndShortcuts` - define access password and shortcut command entries:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>PIN</td>
    <td>string</td>
    <td>
        Access to apps is granted by matching this password PIN at the very beginning of command input.
        <br/>
        See "Usage" for more information.
    </td>
</tr>
<tr>
    <td>Shortcuts</td>
    <td>{"shortcut1":"command1"...}</td>
    <td>Without using password PIN input, these shortcuts are directly translated into the commands and executed.</td>
</tr>
</table>

Optional `TranslateSequences` - translate sequence of command characters to a different sequence:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>Sequences</td>
    <td>[["seq1", "replacement1"]...]</td>
    <td>One after another, character sequences from input command are replaced by the replacements.</td>
</tr>
</table>

Mandatory `LintText` - compact and clean command result:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>CompressSpaces</td>
    <td>true/false</td>
    <td>Compress consecutive space characters into a single space character.</td>
</tr>
<tr>
    <td>CompressToSingleLine</td>
    <td>true/false</td>
    <td>Connect all lines by semi-colon(;) character</td>
</tr>
<tr>
    <td>KeepVisible7BitCharOnly</td>
    <td>true/false</td>
    <td>Only keep Latin letters/digits/symbols and discard letters from other languages.</td>
</tr>
<tr>
    <td>TrimSpaces</td>
    <td>true/false</td>
    <td>Remove leading and trailing space characters.</td>
</tr>
<tr>
    <td>MaxLength</td>
    <td>integer</td>
    <td>Only keep this many characters in the result, discard the remaining ones.</td>
</tr>
</table>

Optional `NotifyViaEmail` - send notification Email for the command input and result:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>Recipients</td>
    <td>array of strings</td>
    <td>Email addresses that will be notified with app commands and command response.</td>
</tr>
</table>

To enable Email notification, please also follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration)
to construct configuration for sending Email responses.

## Configuration example
Here is an example configuration for [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server),
used by both [app command invocation form](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-invoke-app-command)
and [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook):

<pre>
{
    ...

    "HTTPFilters": {
        "PINAndShortcuts": {
            "PIN": "VerySecretPassword",
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
            "Recipients": ["me@example.com", "me2@hotmail.com"]
        }
    },

    ...
}
</pre>

In the example:
- For SMS, `LintText` compacts result and limits length to 160 characters.
- `PINAndShortcuts` has a strong password and three shortcut commands.
- Some dumb phones cannot enter `|` pipe character in SMS, `TranslateSequences` helps them to enter the character
  via `#/` instead.

## Usage
App command looks like:

    PIN .app_identifier parameter1 parameter2 parameter3 ...

Where:
- `.app_identifier` is a short text string that identifies the app to invoke. Pay attention to the leading `.` dot.
- Parameters are passed as-is to the app for interpretation.

Here are the comprehensive list of `.app_identifier` identifiers:

- `.2` - [Two factor authentication code generator](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-two-factor-authentication-code-generator)
- `.a` - [Find text in AES-encrypted files](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-find-text-in-AES-encrypted-files)
- `.c` - [Contact information of public institutions](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-public-institution-contacts)
- `.bp` - [Interactive web browser (PhantomJS)](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-interactive-web-browser-(PhantomJS))
- `.bs` - [Interactive web browser (SlimerJS)](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-interactive-web-browser-(SlimerJS))
- `.e` - [Inspect system and program environment](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-inspect-and-control-server-environment)
- `.g` - [Text search](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-text-search)
- `.i` - [Read Emails](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-reading-Emails)
- `.j` - [Wild joke](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-wild-joke)
- `.m` - [Send Emails](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-sending-Emails)
- `.p` - [Call friends and send texts](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-make-calls-and-send-SMS)
- `.r` - [RSS reader](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-RSS-reader)
- `.s` - [Run system commands](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-run-system-commands)
- `.t` - [Read and post tweets](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-Twitter)
- `.w` - [WolframAlpha](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-WolframAlpha)

### The special "PLT" command
"PLT" is a special command prepended to an ordinary command, in order to seek to position among result output,
and temporarily modify max length and timeout restriction. The usage is:

    PIN .plt <SKIP> <MAX LENGTH> <TIMEOUT SECONDS> .app_identifier parameter1 parameter 2 parameter 3 ...

Where:
- `<SKIP>` is the number of characters to discard from beginning of the result output.
- `<MAX LENGTH>` is the number of characters to respond. It overrides `MaxLength` of `LintText`.
- `<TIMEOUT SECONDS>` is the maximum number of seconds for the app to run. It overrides the usual timeout limit configured in daemons.

Take an example - command `mypassword .il work-mail 0 10`(list 10 Email subjects) is issued to Telegram bot that
gives it 30 seconds to run and restricts output to 76 characters, resulting in the following response:

    1 me@example.com Project roadmap
    2 me@example.com Holiday greetings
    3

Due to combination of `MaxLength` restriction and possible timeout condition, we did not see the remaining 7 Email subjects.
Let us try PLT to retrieve the full output - skip the 2 Emails we have already seen, override `MaxLength` to 10000 and
timeout to 60 seconds:

    .plt 75 10000 60 mypassword .il work-mail 0 10

And we will get the desirable result:

    3 me@example.com Test subject 3
    4 me@example.com Test subject 4
    5 me@example.com Test subject 5
    6 me@example.com Test subject 6
    7 me@example.com Test subject 7
    8 me@example.com Test subject 8
    9 me@example.com Test subject 9
    10 me@example.com Test subject 10

## Tips
Regarding password PIN:
- It must be at least 7 characters long.
- Do not use space character in the password, or it might not be validated successfully during a command invocation.
- Use a strong password that is hard to guess.
- All daemons capable of invoking app commands offer rate limit mechanism to reduce impact of brute-force password guessing.
  Pay special attention to the rate limit settings in individual daemon configuration.
- Each laitos daemon will execute a maximum of 1000 commands per second, regardless of their rate limit configuration.
- Incorrect password PIN entry does not result in an Email notification, however,
  the attempts are logged in warnings and can be inspected via [environment inspection](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-inspect-and-control-server-environment)
  or [program health report](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-program-health-report).

Regarding app command invocation via SMS/telephone:
- Telephone and mobile networks are prone to eavesdroppers, use them only as the last resort.
- The [Twilio SMS/telephone hook](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook)
  is run by the web server daemon and shares command processor configuration from JSON configuration object `HTTPFilters`.
  Check out the link for techniques of command entry via telephone number pad.
- To avoid a high SMS bill, consider turning on all `LintText` flags to compact SMS replies,
  and restrict `MaxLength` to 160 - maximum length of a single SMS text.
- Some mobile phones using pre-2007 design cannot input the pipe character `|` that is commonly used in system shell commands.
  To work around the issue, configure a `TranslateSequences` such as `["#/", "|"]`.

Regarding mail notification and logging: the input of 2FA code generator and AES-encrypted content search are concealed
from all mail notifications and log messages in order to protect their encryption key.
