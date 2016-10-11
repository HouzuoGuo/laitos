Websh
=====
A simple do-everything daemon, primary for offering control of your computer via telephone and SMS.

It can do:
- Run shell commands and get output in text form.
- Run WolframAlpha queries and get result in text form.
- Truncate response/output to shorter length.
- Define shortcuts (preset messages) for long commands.
- Invoke Twilio to call a number (and speak a text) or send a text message.
- Deliver all of the features by responding to HTTP requests or incoming Emails.
- HTTP daemon can also act as Twilio web-hook (both voice and message) to allow running commands via telephone call and text messages.

Good for emergency system shutdown/reboot, and executing privileged/unprivileged shell code.

Please note: exercise _extreme caution_ when using this software program, inappropriate configuration will make your computer vulnerable to attacks! I will not be responsible for any damage/potential damage caused to your computers.

Build
=================
Simply run "go build".

Run HTTP daemon
========
You need the following:

- HTTPS certificate and key
- Secret API endpoint names (longer is better)
- A secret PIN (longer is better)

Create a JSON configuration file:

<pre>
{
    "MessageEndpoint": "my_secret_endpoint_name_without_leading_slash",
    "VoiceMLEndpoint": "twilio_hook_initial_contact_without_leading_slash",
    "VoiceProcEndpoint": "twilio_hook_processor_without_leading_slash",
    "VoiceEndpointPrefix": "/optional_voice_hook_proxy_prefix",
    "ServerPort": 12321,
    "PIN": "MYSECRET",
    "TLSCert": "/tmp/test.crt",
    "TLSKey": "/tmp/test.key",

    "SubHashSlashForPipe": true,
    "WebTimeoutSec": 10,
    "WebTruncateLen": 120,

    "MailTimeoutSec": 20,
    "MailTruncateLen": 240,
    "MailRecipients": ["ITsupport@mydomain.com"],
    "MailFrom": "admin@mydomain.com",
    "MailAgentAddressPort": "mydomain.com:25",

    "PresetMessages": {
        "secretapple": "echo hello world",
        "secretpineapple": "poweroff"
    },

    "WolframAlphaAppID": "optional-your-wolframalpha-app-id",

    "TwilioNumber": "optional-twilio-outgoing-originating-number",
    "TwilioSID": "optional-twilio-outgoing-account-sid",
    "TwilioAuthSecret": "optional-twilio-outgoing-account-secret",

    "MysteriousURL": "",
    "MysteriousAddr1": "",
    "MysteriousAddr2": "",
    "MysteriousID1": "",
    "MysteriousID2": ""
}
</pre>

Run the executable with command line:

    ./websh -configfilepath=/path/to/config.json

Invoke the API service from command line:

    curl -v 'https://localhost:12321/my_secret_endpoint_name_without_leading_slash' --data-ascii 'Body=MYSECRETecho hello world'

Please note that:

- Email notifications can be optionally enabled to send shell statement results to the specified recipients (MailRecipients) in addition to the web API response. To enable the notifications, fill in all mail parameters.
- Mail FROM and recipients must be use full address(name@domain.net), MTA address must contain both host name (domain name) and port number.
- If there is a PIN mismatch, the response code will be 404.
- The API endpoint looks for PIN and shell statement together, in form parameter "Body".
- Do not insert extra space(s) between the secret PIN and your shell statement.
- To invoke WolframAlpha query, set WolframAlphaAppID and then use prefix "#w" immediately following PIN in the incoming message.
- MessageEndpoint conforms to Twilio SMS web-hook, make sure to "-outtrunclen" to avoid sending too many SMS responses.
- Voice endpoints are optional, fill in if you need to run Twilio voice web-hook. On Twilio, the web-hook configuration URL should use `VoiceMLEndpoint`.

There is also an example systemd unit file that can help with running the program as a daemon.

Run Email processor
==================
The program has a "mail mode" that processes shell statements from incoming mails, instead of running as a stand-alone daemon, secured by the identical PIN-matching mechanism.

To run in mail mode, specify all mail-related parameters in the configuration file, and enable mail processing by creating ".forward" file in the home of your user of choice, with the following content:

<pre>
\my_user_name
"|/abspath/to/websh_executable -mailmode=true -configfilepath=/path/to/config.json"
</pre>

The first line makes sure that incoming mails are always delivered to mailbox. The second line pipes incoming mails to this program, running in "mail mode".

Here is an example of invoking the mail-shell using mailx command:

    echo 'MYSECRETecho hello world' | mail my_user_name@mydomain.com -s subject_does_not_matter

Shell statement output and execution result will be mailed to sender my\_user\_name@mydomain.com.

Run HTTP daemon as Twilio web-hook
===================
The program supports Twilio voice web-hook so that you can run shell commands by typing in a sequence of keys on keypad, and have the command output read out to you. To set it up:

- Please carefully read the DTMF sections of the source code (`voiceDecodeDTMF`) to understand how keypad input is intepreted. In general: asterisk switches letter case, zero marks end of a numeral or symbol, zeros followed by a zero give spaces, and key one can type various symbols and numbers.
- The Twilio parameters in JSON configuration are only for outgoing calls and texts, they do not influence the operation of web-hook, which deals with incoming calls and texts.
- As of July 2016, Twilio voice hook does not support port number in the URL, if you decide to run this program on a port different from 443, please place a proxy (such as apache HTTP server) in front so that this program can be accessed via HTTPS without special port number.
- If there is a proxy in front of the voice API endpoints and the proxy places additional path segments the endpoints (e.g. proxy directs `/voice/my_hook` at `/my_hook`), please enter the additional path segments in `VoiceEndpointPrefix` (e.g. `/voice/`).
- If there is not a proxy in front of the voice API endpoints, set `VoiceEndpointPrefix` to a single forward slash (`/`).
- On Twilio configuration panel, the web-hook URL should use `VoiceMLEndpoint`, which is the initial contact point. `VoiceProcEndpoint` is not relevant to your Twilio configuration and can be an arbitrary string of letters.

After the voice web-hook has been set up, dial your Twilio number and enter PIN as you would with message-shell and mail-shell, followed by the command to run. Command output will be dictated back to you, terminating with the word "over", then you may enter a new command and the cycle repeats untill you hang up.

If PIN entry is incorrect, voice will say sorry and hang up. The usual logging and mail notifications apply to voice-shell.

Copyright
====================
Copyright (c) 2016, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
