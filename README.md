Websh
=====
A simple web server daemon enabling basic shell access via API calls.

Good for emergency system shutdown/reboot, and executing privileged/unprivileged shell code.

Please note: exercise _extreme caution_ when using this software program, inappropriate configuration will make your computer easily compromised! If you choose to use this program, I will not be responsible for any damage/potential damage caused to your computers.

Build
=================
Run "go build" to build the executable.

Running
========
You need the following:

- HTTPS certificate and key
- A secret API endpoint name (longer is better)
- A secret PIN (longer is better)

Create a JSON configuration file:

<pre>
{
    "MessageEndpoint": "my_secret_endpoint_name_without_leading_slash",
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

    "MysteriousURL": "",
    "MysteriousAddr1": "",
    "MysteriousAddr2": "",
    "MysteriousID1": "",
    "MysteriousID2": "",
    "WolframAlphaAppID": "your-wolframalpha-app-id"
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
- The API endpoint can be used as Twilio SMS web-hook. Make sure to shorten "-outtrunclen" to avoid sending too many SMS responses.
- To invoke WolframAlpha query, set WolframAlphaAppID and then use prefix "#w" immediately following PIN in the incoming message.

There is also an example systemd unit file that can help with running the program as a daemon.

Running mail-shell
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

Copyright
====================
Copyright (c) 2016, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
