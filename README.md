Websh
=====
A simple web server daemon enabling basic shell access via API calls.

Good for emergency system shutdown/reboot, and executing privileged/unprivileged shell code.

Build
=================
Run "go build" to build the executable.

Test Run
========
You need the following:

- HTTPS certificate and key
- A secret API endpoint name (longer is better)
- A secret PIN (longer is better)

Run the executable with command line:

    ./websh -endpoint=SecretAPIEndpointName -pin=SecretPIN -tlscert=tls.crt -tlskey=tls.key -port=12321 -mailfrom=root -mailrecipients=me@example.com -mtaaddr=127.0.0.1 -cmdtimeoutsec=10 -outtrunclen=120

Invoke the API service from command line:

    curl -v 'https://localhost:12321/SecretAPIEndpointName' --data-ascii 'Body=SecretPINecho hello world'

Please note that:

- If there is a PIN mismatch, the response code is 404.
- The API endpoint looks for PIN and shell command together, in form parameter "Body".
- Do not insert extra space(s) between the secret PIN and your shell command.
- The API endpoint can be used as Twilio SMS web-hook. Make sure to shorten "-outtrunclen" to avoid sending too many SMS responses.

Production Run
==============
Edit systemd.unit to adjust executable path, run-as user, and place the unit file in /etc/systemd/system/. Enable the unit and enjoy!

Copyright and Author
====================
Copyright (c) 2016, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
