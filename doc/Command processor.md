# Command processor

## Introduction
The following daemon components have an embedded command processor to let users use toolbox features:
- [Mail server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-mail-server)
- [Telegram chat-bot](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-telegram-chat-bot)
- [Plain-text sockets](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-plain-text-sockets)
- Web service [invoke toolbox features in a form](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-toolbox-features-form)
- Web service [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-telephone-and-SMS-hook-with-Twilio)
- Web service [Microsoft bot hook](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-Microsoft-bot-hook)

To use toolbox feature via a daemon, the following events take place:
1. Input a command. For example, web server collects input in an HTML form, and mail server collects input from incoming
   mail content.
2. Filter command through `PINAndShortcuts` mechanism - match access password (PIN) and translate shortcut entries.
3. Filter it further through `TranslateSequences` mechanism - replace sequence of characters by another sequence.
4. Execute toolbox feature identified by the `prefix` name, and give the parameters to the toolbox feature as context.
   Once done, the result is presented in an easy-to-read text.
5. Filter the result through `LintText` mechanism - compact and clean result text when necessary.
6. If result is empty, inform user by replacing it to `EMPTY OUTPUT`.
7. Notify user the command input and result via Email.

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
        Access to toolbox is granted only after this strong password PIN is matched at the beginning of command input.
        <br/>
        See "Usage" for more information.
    </td>
</tr>
<tr>
    <td>Shortcuts</td>
    <td>{"shortcut1":"command1"...}</td>
    <td>Without requiring PIN input, these shortcuts are directly translated into the commands and executed.</td>
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
    <td>Email addresses that will receive the notification.</td>
</tr>
</table>

To enable Email notification, please also follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration)
to construct configuration for sending Email responses.

## Configuration example
Here is an example configuration for [web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server),
used by both [HTML toolbox form](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-toolbox-features-form)
and [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-telephone-and-SMS-hook-with-Twilio):

<pre>
{
    ...

    "HTTPFilters": {
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
            "CompressSpaces": true,
            "CompressToSingleLine": true,
            "KeepVisible7BitCharOnly": true,
            "MaxLength": 160,
            "TrimSpaces": true
        },
        "NotifyViaEmail": {
            "Recipients": ["howard@gmail.com", "howard@hotmail.com"]
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
A command issued to toolbox feature looks like this:

    PIN .feature_prefix parameter1 parameter2 parameter3 ...

Where:
- `.feature_prefix` tells which toolbox feature is to be executed. Pay attention to the leading `.` dot.
- parameters are passed on to the feature for execution.

The following prefixes are accepted, see individual feature manual for their usage:

- `.2` - [Two factor authentication code generator](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-two-factor-authentication-code-generator)
- `.a` - [Find text in AES-encrypted files](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-find-text-in-AES-encrypted-files)
- `.b` - [Interactive web browser](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-interactive-web-browser)
- `.e` - [Inspect system and program environment](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-inspect-environment)
- `.f` - [Facebook](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-Facebook)
- `.i` - [Read Emails](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-reading-Emails)
- `.m` - [Send Emails](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-sending-Emails)
- `.p` - [Call friends and send texts](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-telephone-and-SMS)
- `.s` - [Run system commands](https://github.com/HouzuoGuo/laitos/wiki/Toolbox:-run-system-commands)
- `.t` - [Read and post tweets](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-Twitter)
- `.w` - [WolframAlpha](https://github.com/HouzuoGuo/laitos/wiki/Toolbox-feature:-WolframAlpha)

### The special "PLT" command
"PLT" is a special command prepended to an ordinary command, in order to seek to position among result output,
and temporarily modify max length and timeout restriction. The usage is:

    PIN .plt <SKIP> <MAX LENGTH> <TIMEOUT SECONDS> .feature_prefix parameter1 parameter 2 parameter 3 ...

Where:
- `<SKIP>` is the number of characters to discard from beginning of the result output.
- `<MAX LENGTH>` is the number of characters to respond. It overrides `MaxLength` of `LintText`.
- `<TIMEOUT SECONDS>` is the number of seconds toolbox feature may run without being aborted. It overrides usual timeout
  limit configured in daemon.

Take an example - command `MY_TOOLBOX_PIN .il work-mail 0 10`(list 10 Email subjects) is issued to Telegram bot that
gives it 30 seconds to run and restricts output to 76 characters, resulting in the following response:

    1 howard@gmail.com Project roadmap
    2 howard@hotmail.com Holiday greetings
    3

Due to combination of `MaxLength` restriction and possible timeout condition, we did not see the remaining 7 Email subjects.
Let us try PLT to retrieve the full output - skip the 2 Emails we have already seen, override `MaxLength` to 10000 and
timeout to 60 seconds:

    .plt 75 10000 60 MY_TOOLBOX_PIN .il work-mail 0 10

And we will get the desirable result:

    3 howard@gmail.com Test subject 3
    4 howard@gmail.com Test subject 4
    5 howard@gmail.com Test subject 5
    6 howard@gmail.com Test subject 6
    7 howard@gmail.com Test subject 7
    8 howard@gmail.com Test subject 8
    9 howard@gmail.com Test subject 9
    10 howard@gmail.com Test subject 10

## Tips
Regarding password PIN:
- Must be at least 7 characters long.
- Do not use space character in the password; otherwise the space characters will cause most features to misbehave.
- Use a strong password to protect access to toolbox features.
- Every daemon that has a command processor also has a rate limit mechanism (e.g. `PerIPLimit` configuration),
  avoid setting rate limit too high or password may be prone to brute-force attack.
- Incorrect password entry does not result in an Email notification, however,
  the attempts are logged in warnings and can be inspected via [environment inspection](https://github.com/HouzuoGuo/laitos/wiki/Toolbox:-inspect-environment)
  or [health report](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-health-report).

Regarding toolbox usage via SMS/telephone:
- Telephone and mobile networks are prone to attacks, they can eavesdrop your password PIN and toolbox feature
  conversations easily. Use them only as a last resort.
- The [Twilio SMS/telephone hook](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-telephone-and-SMS-hook-with-Twilio)
  runs in the web server daemon, therefore the corresponding command processor configuration is in JSON key `HTTPFilters`.
  Check out the feature's manual for techniques of command entry via telephone number pad.
- To avoid a high SMS bill, consider turning on all `LintText` flags to compact SMS replies,
  and restrict `MaxLength` to 160 - maximum length of a single SMS text.
- Some mobile phones using pre-2007 design cannot input the pipe character `|` that is commonly used in system commands.
  To work around the issue, configure a `TranslateSequences` such as `["#/", "|"]`.