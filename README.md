[![Build Status](https://travis-ci.org/HouzuoGuo/laitos.svg?branch=master)](https://travis-ci.org/HouzuoGuo/laitos)

laitos
=====
laitos has many Internet infrastructure daemons built into a single executable, they also enable users to access Internet features via alternative means such as PSTN (telephone), GSM (SMS), and satellite messaging.

laitos was called "websh" before 13th March 2017.

The latest daemon features are:
- Generic purpose HTTP daemon serves static HTML pages, directories, Twilio hooks, and more.
- Telegram bot serves Internet features via telegram chats.

The latest Internet features are:
- Decrypt and find lines among AES-encrypted content.
- Post to Facebook.
- Send Emails.
- Run shell commands.
- Call a phone number and speak some texts.
- Send text message to a mobile phone number.
- Post to Twitter.
- Retrieve latest tweets.
- Run WolframAlpha queries.

You must exercise _extreme caution_ when using this software program, inappropriate configuration will severely compromise the security of the host computer. I am not responsible for any damage/potential damage caused to your computers.

Build
=====
Simply check out the source code under your `$GOPATH` directory and run `go build`.

The program only uses Go's standard library, it does not depend on 3rd party libraries.

Configuration
=============
TODO

Copyright
====================
Copyright (c) 2017, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
