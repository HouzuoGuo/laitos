# Daemon: telegram chat bot

## Introduction
Telegram Messenger is a popular mobile messaging app that excels in communication security.

The chat bot enables you to run toolbox feature commands via chat messages.

## Preparation
Acquire an `AuthorizationToken` from Telegram Messenger by creating your own chat bot. Download and install Telegram
Messenger, then follow the guide [How do I create a bot?](https://core.telegram.org/bots) to talk to "BotFather" and
register your chat bot.

After chat bot is successfully created, Telegram Messenger will tell you the `AuthorizationToken`, which you have to
place into the configuration.

## Configuration
1. Construct the following JSON object and place it under JSON key `TelegramBot` in configuration file.
   The following properties are mandatory:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>AuthorizationToken</td>
    <td>string</td>
    <td>Registered bot authorisation token.</td>
</tr>
<tr>
    <td>RateLimit</td>
    <td>integer</td>
    <td>Maximum number of toolbox commands the chat bot will process in a ten-second interval.</td>
</tr>
</table>

2. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `TelegramFilters`.

Here is an example setup of Telegram:
<pre>
{
    ...
    
    "TelegramBot": {
        "AuthorizationToken": "425712345:ABCDEFGHIJKLMNOPERSTUVWXYZ",
        "Port": 25
        "PerIPLimit": 3,
        
        "ForwardTo": ["howard@gmail.com", "howard@hotmail.com"],
        "MyDomains": ["howard-homepage.net", "howard-blog.org"],
        
        "TLSCertPath": "/root/howard-blog.org.crt",
        "TLSKeyPath": "/root/howard-blog.org.key"
    },
    "TelegramFilters": {
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
            "CompressSpaces": false,
            "CompressToSingleLine": false,
            "KeepVisible7BitCharOnly": false,
            "MaxLength": 4096,
            "TrimSpaces": false
        },
        "NotifyViaEmail": {
            "Recipients": ["howard@gmail.com"]
        }
    },
    
    ...
}
</pre>

## Run
Tell laitos to run chat bot daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,telegram,...

## Usage
On Telegram Messenger application, find your newly created chat bot via the in-app Search function, then send toolbox
command in a chat message. Wait a short moment, and the command response will be sent back to you via the same chat.

Don't forget to put password PIN in front of the toolbox command!

## Tips
The chat bot server will not process messages that arrived before the server started, which means, you cannot leave a
message to the chat bot while server is offline.