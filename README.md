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
Copyright (C) 2017 Howard (Houzuo) Guo <guohouzuo@gmail.com>

This program is free software:
you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation,
either version 3 of the License, or (at your option) any later version.
This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
See the GNU General Public License for more details.
You should have received a copy of the GNU General Public License along with this program.
If not, see <http://www.gnu.org/licenses/>.