[![Build Status](https://travis-ci.org/HouzuoGuo/websh.svg?branch=master)](https://travis-ci.org/HouzuoGuo/websh)

Websh
=====
A comprehensive do-everything daemon, delivers all of the following features via telephone calls, SMS, email exchange, and web API calls:
- Run shell commands and retrieve result.
- Run WolframAlpha queries and retrieve result.
- Call another telephone number and speak a piece of text.
- Send SMS to another person.
- Post to Twitter time-line.
- Read the latest updates from Twitter home time-line.
- Post Facebook update.

Please note: exercise _extreme caution_ when using this software program, inappropriate configuration will make your computer vulnerable to attacks! I will not be responsible for any damage/potential damage caused to your computers.

Build
=====
Simply check out the source code under your `$GOPATH` directory and run `go build`.

The program does not use anything other than Go standard library.

Configuration
=============
The program uses a single JSON configuration file that is specified via command line parameter. The configuration is quite long, but feel free to leave blank in those features that you are not going to use.
<pre>
{
</pre>
- For all use cases, prepare a secret PIN (alphanumeric characters). All incoming messages must begin with the correct PIN to authorise command execution. Specify a long and strong PIN to prevent unauthorised access:
<pre>
    "PIN": "MYSECRET",
</pre>
- To serve Twilio phone number hook via web API, prepare HTTPS certificate and key and write down these configurations in JSON. Voice hook parameters are explained in detail in a later section. All endpoint names should be difficult to guess:
<pre>
    "MessageEndpoint": "my_secret_endpoint_name_without_leading_slash",
    "VoiceMLEndpoint": "twilio_hook_initial_contact_without_leading_slash",
    "VoiceProcEndpoint": "twilio_hook_processor_without_leading_slash",
    "VoiceEndpointPrefix": "/optional_voice_hook_proxy_prefix",
    "ServerPort": 12321,
    "TLSCert": "/tmp/test.crt",
    "TLSKey": "/tmp/test.key",
</pre>
- When running as a phone number hook, to work around certain dumb phones that cannot type pipe symbol in SMS, they may use a pound sign and forward slash together (`#/`) to substitute for a pipe:
<pre>
    "SubHashSlashForPipe": true,
</pre>
- Running commands via web API call and phone number hooks will be constrained by execution timeout and truncated output length. Set truncated output length appropriately to avoid sending too many SMS responses in a phone number hook:
<pre>
    "WebTimeoutSec": 10,
    "WebTruncateLen": 120,
</pre>
- Configure Email parameters for sending notifications after each command execution and/or running as a mail processor. Mail FROM and recipients must be use full address(name@domain.net), MTA address must contain both host name (domain name) and port number.:
<pre>
    "MailRecipients": ["ITsupport@mydomain.com"],
    "MailFrom": "admin@mydomain.com",
    "MailAgentAddressPort": "mydomain.com:25",
</pre>
- Running commands via Email exchange is constrained by independent (usually relaxed) execution timeout and truncated output length:
<pre>
    "MailTimeoutSec": 20,
    "MailTruncateLen": 240,
</pre>
- Use this application ID to query WolframAlpha (e.g. `MYSECRET#w weather in Nuremberg, Germany`):
<pre>
    "WolframAlphaAppID": "your-wolframalpha-app-id",
</pre>
- Use this user access token to post Facebook status updates (e.g. `MYSECRET#f hello there facebook friends`). Remember to use Facebook's "long lived access token", and refresh the token once every 30 days!
<pre>
    "FacebookAccessToken": "your-facebook-user-access-token",
</pre>
- Use this Twilio application credential to making outgoing calls (e.g. `MYSECRET#c +4912345 message to speak`) and SMS (e.g. `MYSECRET#s +4912345 message to send`):
<pre>
    "TwilioNumber": "twilio-outgoing-originating-number",
    "TwilioSID": "twilio-outgoing-account-sid",
    "TwilioAuthSecret": "twilio-outgoing-account-secret",
</pre>
- Use this Twitter credential to retrieve home time-line (e.g. read 30 latest tweets, drop the latest 12 `MYSECRET#tg 12 30`) and post tweet (e.g. `MYSECRET#tp This is a timeline update`):
<pre>

	"TwitterConsumerKey": "twitter-app-consumer-key",
	"TwitterConsumerSecret": "twitter-app-consumer-secret",
	"TwitterAccessToken": "twitter-app-user-access-token",
	"TwitterAccessSecret": "twitter-app-user-access-secret",
</pre>
- Define secret shortcuts that execute commands without requiring PIN entry:
<pre>
    "PresetMessages": {
        "secretapple": "echo hello world",
        "secretpineapple": "poweroff"
    }
</pre>
<pre>
}
</pre>

Web API/phone number hook daemon
===============================
Run the executable with command line:

    ./websh -configfilepath=/path/to/config.json

To test run the API service, use curl from command line:

    curl -v 'https://localhost:12321/my_secret_endpoint_name_without_leading_slash' --data-ascii 'Body=MYSECRETecho hello world'

General notes:

- Write PIN characters in front of the command to run. If there is a PIN mismatch, the HTTP response code will be 404.
- "Body" is the parameter in which the API looks for PIN and command statement. It is not necessary to insert extra space(s) between the PIN and command statement.
- Use prefix `#o skip_len output_len` to skip certain number of characters in command response, and temporary override response length restriction to the specified value.

Notes for Twilio phone-number hook:

- Please carefully read the DTMF sections of the source code (`dtmf.go`) to understand how keypad input is interpreted. In general: `*` switches letter case, `0` marks end of a numeral or symbol, zeros followed by a zero give spaces, and `1` types various symbols and numbers.
- As of December 2016, Twilio voice hook does not support port number in the URL, if you decide to run this program on a port different from 443, please place a proxy (such as apache HTTP server) in front so that this program can be accessed via HTTPS without special port number.
- If there is a proxy in front of the voice API endpoints and the proxy places additional path segments the endpoints (e.g. proxy directs `/voice/my_hook` at `/my_hook`), please enter the additional path segments in `VoiceEndpointPrefix` (e.g. `/voice/`).
- If there is not a proxy in front of the voice API endpoints, set `VoiceEndpointPrefix` to a single forward slash (`/`).
- On Twilio configuration panel, the web-hook URL should use `VoiceMLEndpoint`, which is the initial contact point. `VoiceProcEndpoint` is not relevant to your Twilio configuration and can be an arbitrary string of letters.
- Twilio times out its HTTP request after 15 seconds, so make sure to lower `WebTimeoutSec` below 15.
- After the voice web-hook has been set up, dial your Twilio number and enter PIN as you would normally do, followed by the command to run. Command output will be dictated back to you, terminating with the word "over", then you may enter another PIN + command and the cycle repeats until you hang up. Always enter PIN before entering each command, even though you have already entered the correct PIN in a previous command.
- If PIN entry is incorrect, voice will say sorry and hang up. The usual logging and mail notifications apply to phone number hooks.

There is also an example systemd unit file that can help with running the program as a daemon.

Mail processor
==============
The program has a "mail processor mode" that processes commands from user's incoming mails, secured by the identical PIN-matching mechanism.

Make sure to specify all mail parameters in the configuration file for this mode to work.

To enable, create a `.forward` file in the home directory of user, with the following content:

<pre>
\my_user_name
"|/abspath/to/websh_executable -mailmode=true -configfilepath=/path/to/config.json"
</pre>

The first line tells mail transfer agent (e.g. postfix or sendmail) to put incoming mail into user's mail box. The second line pipes the content of incoming mail to this program. The program will try to match PIN and find a single command to execute in the body of mail (subject is discarded).
Once processing is finished, the program delivers command response back to the sender using the mail parameters specified in configuration file.

After the mail processor is set up, test run it using this command:

`echo 'MYSECRETecho hello world' | mail my_user_name@mydomain.com -s subject_does_not_matter`

Moments later, observe command response in the mail box of `my_user_name@mydomain.com`.

Copyright
====================
Copyright (c) 2016, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
