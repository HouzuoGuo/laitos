# Command processor

## Introduction

The following daemon components are capable of executing app commands:

- [DNS server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server)
- [Mail server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-mail-server)
- [Telnet server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telnet-server)
- [Telegram chat-bot](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telegram-chat-bot)
- [Phone home telemetry](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry)
- Web service [app command form](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-invoke-app-command)
- Web service [simple app command execution API](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-simple-app-command-execution-API)
- Web service [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook)
- Web service [Microsoft bot hook](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Microsoft-bot-hook)
- Web service [The Things Network LORA tracker integration](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-the-things-network-LORA-tracker-integration)

During app command invocation, the following actions take place:

1. The user enters a command, for example, by using the "invoke app command" web service form, or by sending an app command
   in an Email addressed to laitos mail server. (e.g. `mypass .e info`)
2. laitos validates the password from the input command to match configuration from `PINAndShortcuts`, or if the input is a
   shortcut, laitos expands the shortcut into full command without looking for a password.
3. laitos walks the app command (excluding the password portion) through `TranslateSequences` mechanism that replaces sequence
   of characters by a different sequence.
4. laitos identifies the app (e.g. `.e` for program control) and gives the app remainder of the command input for parameters.
5. The app routine runs and produces plain text response.
6. laitos walks the text response through `LintText` mechanism that compacts and tidies up the text if needed. As a special case,
   if the app produces an empty response, the actual app response will change to `EMPTY OUTPUT`.
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
    <td>Passwords</td>
    <td>array of strings</td>
    <td>
        Put any of the passwords at the very beginning of app command input to gain access to apps.
        A password consists of letters and numbers. Avoid using spaces and special symbols.
        <br/>
        See "Usage" for more information.
    </td>
</tr>
<tr>
    <td>Shortcuts</td>
    <td>{"shortcut1":"command1"...}</td>
    <td>Without using password input, these shortcuts are directly translated into the commands and executed.</td>
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

Mandatory `LintText` - compact and clean up command output text:

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>CompressSpaces</td>
    <td>true/false</td>
    <td>
      Replace consecutive space characters into a single space character.
      This helps to compact the output text.
    </td>
</tr>
<tr>
    <td>CompressToSingleLine</td>
    <td>true/false</td>
    <td>
      Repalce line breaks with a semi-colon (;) character.
      This helps to display the output on devices unable to display multi-line text.
    </td>
</tr>
<tr>
    <td>KeepVisible7BitCharOnly</td>
    <td>true/false</td>
    <td>
      Retain Latin letters, digits, and symbols, discard the rest such as Cyrillic and CJK characters.
      This helps to display the output on devices that are unable to display richer character sets.
    </td>
</tr>
<tr>
    <td>TrimSpaces</td>
    <td>true/false</td>
    <td>
      Remove leading and trailing space characters.
      This helps to compact the output text.
    </td>
</tr>
<tr>
    <td>MaxLength</td>
    <td>integer</td>
    <td>Maximum number of characters to retain in the command output. Remaining text is discarded.</td>
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
    <td>These Email addresses will receive a message with each input app command and command response.</td>
</tr>
</table>

To enable Email notification, please also follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration)
to construct configuration for sending Email responses.

In order to protect encryption secret, the notification Email will hide the input command for laitos 2FA code generator app and
AES-encrypted text search app, though the result (2FA codes and encrypted text search result) will still appear in the Email mesage.

## Configuration example

Here is an example configuration for [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server),
used by both [app command invocation form](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-invoke-app-command)
and [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook):

<pre>
{
    ...

    "HTTPFilters": {
        "PINAndShortcuts": {
            "Passwords": ["VerySecretPassword", "SecretPasswordPineapple"],
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
- `PINAndShortcuts` defines two passwords, both of which will authorise app commands to execute; it also defines three shortcuts - each
  translates into a command without having to enter the password.
- Certain old mobile phones cannot enter the pipe character `|` in an SMS, `TranslateSequences` helps those phones to enter a pipe character
  via combo `#/` instead.

## Usage

App command looks like:

    Password .app_identifier parameter1 parameter2 parameter3 ...

Where:

- `Password` is one of the passwords from the `Passwords` array of valid password strings.
- `.app_identifier` is a short text string that identifies the app to invoke. Pay attention to the mandatory leading `.` dot.
- Parameters are passed as-is to the specified app as its input.

Here are the comprehensive list of `.app_identifier` identifiers:

- `.2` - [Two factor authentication code generator](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-two-factor-authentication-code-generator)
- `.a` - [Find text in AES-encrypted files](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-find-text-in-AES-encrypted-files)
- `.c` - [Contact information of public institutions](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-public-institution-contacts)
- `.e` - [Inspect system and program environment](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-inspect-and-control-server-environment)
- `.g` - [Text search](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-text-search)
- `.i` - [Read Emails](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-reading-Emails)
- `.j` - [Wild joke](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-wild-joke)
- `.m` - [Send Emails](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-sending-Emails)
- `.p` - [Call friends and send texts](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-make-calls-and-send-SMS)
- `.r` - [RSS reader](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-RSS-reader)
- `.s` - [Run system commands](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-run-system-commands)
- `.w` - [WolframAlpha](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-WolframAlpha)

### Use one-time-password in place of password

If you become concerned of eavesdroppers that might maliciously intercept the password, consider using one-time-password in place of
password in an app command input, this technique can be used with any of the passwords defined in `PINAndShortcuts` follow these steps:

1. Ensure that the password defined in `PINAndShortcuts` contains only lowercase letters, otherwise OTP will not work.
2. Download and install a two factor authentication code generator, such as [Authy](https://authy.com/) app.
3. In the app, create a new time-based account, name it "LaitosOTP1", and instead of scanning a QR code, manually enter the password
   for the secret.
4. In the app, create another time-based account, name it "LaitosOTP2", and instead of scanning a QR code, manually enter the reversed
   password for the secret.

Back to laitos, enter an app command this way: `LaitosOTP1LaitosOTP2 .app_identifier ...`. Each OTP has 6 digits, put the two OTPs
together with no space in between. Should a malicious eavesdropper intercept the communication, the eavesdropper will not be able to
recover your password from the OTPs.

Enforced by laitos server, a combination of two OTPs may only be used with one app command until the OTPs expire, for example, if that
OTP 1 is `123123` and OTP 2 is `789789`:

- User may repeatedly execute app command `123123789789 .s echo hello` arbitrary number of times via any daemon.
- User may not execute command `123123789789 .s echo hello` and then `123123789789 .s echo hi`, the first command will succeed but laitos
  will refuse to execute the second "echo hi" command by saying "the TOTP has already been used with a different command".

### Override output length and timeout restriction

By default, daemons that are capable of receiving app commands, executing them, and respond with execution result will impose several
restrictions on:

- Maximum length of the app command input.
- Maximum length of the app command execution result (output).
- Maximum duration (in seconds) that the app command execution may take.

The restrictions are often configurable, and they are crucial in preventing a malfunctioning app command from exhausting server resources.
However, in certain cases, you may wish to temporarily lift the restriction for an individual app command, for example when executing an
app command over SMS the response is usually restricted to ~130 characters (max. 1 SMS), and by using the special `.plt` string, you may
then read an a response 300 characters long (in 3 texts).

An app command that uses `.plt` string looks like:

    PasswordPIN .plt SKIP MaxLength TimeoutSeconds .app_identifier parameter1 parameter 2 parameter 3 ...

Where:

- `PasswordPIN` is the password PIN to authorise app command execution.
- `SKIP` is the number of characters to discard from beginning of the result output.
- `MaxLength` is the maximum number of characters to collect from command response. It overrides `MaxLength` of `LintText`, or the default
  limit imposed by a daemon internally.
- `TimeoutSeconds` is the maximum number of seconds the app may spend to execute the command. It overrides daemon's internal default limit.

Take an example - a user uses the Telegram bot daemon to execute command `mypassword .il work-mail 0 10` (get the latest 10 Email subjects).
The user previously configured `LintText` to restrict output to only 76 characters, and Telegram bot internally spends at most 30 seconds to
execute a command. These constraints would result in this incomplete response:

    1 me@example.com Project roadmap
    2 me@example.com Holiday greetings
    3

Let us try to retrieve the full output - skip the 2 Email subjects already seen (the first 75 characters), then override maximum output length
to 10000 and timeout to 60 seconds:

    mypassword .plt 75 10000 60 .il work-mail 0 10

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

Regarding password:

- It must be at least 7 characters long. Use a strong password that is hard to guess.
- Feel free to use numbers, upper and lower case letters, but please refrain from using other special characters or space characters in the password.
- All daemons capable of invoking app commands offer rate limit mechanism to reduce impact of brute-force password guessing. Pay special
  attention to the rate limit settings in individual daemon configuration.
- For prevention of brute-force guessing of password via DDoS, laitos enforces a global limitation of maximum 1000 command executions per second,
  regardless of how many daemons are processing the app commands.
- Incorrect password PIN entry does not result in an Email notification, however, the attempts are logged in warnings and can be inspected via
  [environment inspection](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-inspect-and-control-server-environment)
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
