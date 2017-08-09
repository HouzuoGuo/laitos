# Command processor

## Introduction
Most daemon components have an embedded command processor to let users use toolbox features.

For example, the web service [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-telephone-and-SMS-hook-with-Twilio) runs in web server daemon,
and it helps you to use toolbox features via telephone calls and SMS texts.

In order for a user to use toolbox feature via a daemon, the following events take place:
1. Collect command input that looks like `.prefix parameter1 parameter 2...`.
For example, web server collects input in an HTML form, and mail server collects input from incoming mail content.
2. Filter command through `PINAndShortcuts` mechanism - match access password (PIN) and translate shortcut entries.
3. Filter it further through `TranslateSequences` mechanism - replace sequence of characters by another sequence.
4. Execute toolbox feature identified by the `prefix` name, and give the parameters to the toolbox feature as context.
Once done, the result is presented in an easy-to-read text.
5. Filter the result through `LintText` mechanism - compact and clean result text when necessary.
6. If result is empty, inform user by replacing it to `EMPTY OUTPUT`.
7. Notify user the command input and result via Email. 

## Configuration

Construct the following objects under configuration key (e.g. `HTTPBridges`, `MailBridges`) named by individual daemon - you may find them in daemon's usage manual.

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

Mandatory `TranslateSequences` - translate sequence of command characters to a different sequence:
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


## Configuration example

Here is an example configuration for [web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server), used by both [HTML toolbox form](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-toolbox-features-form) and [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-telephone-and-SMS-hook-with-Twilio):

<pre>
{
    ...

    "HTTPBridges": {
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
- Some dumb phones cannot enter `|` pipe character in SMS, `TranslateSequences` helps them to enter the character via `#/` instead.

## Usage

## Tips
Regarding password PIN:
- Must be at least 7 characters long.
- Use a strong password to protect access to toolbox features.
