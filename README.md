[![Build Status](https://travis-ci.org/HouzuoGuo/laitos.svg?branch=master)](https://travis-ci.org/HouzuoGuo/laitos)

# laitos - empower your personal web server
Want to host a simple website, Email server, or ad-blocking DNS server? Skip those scary tutorials you find online!

<strong>laitos</strong> has all your needs covered - plus, it offers You access to mails and social networks via fun ways such as telephone call, SMS, Telegram chat, and even satellite messenger!

## Highlights

- <strong>Powerful</strong> - web, mail and DNS servers beautifully re-invented in just 9K lines of code.
- <strong>Fun</strong> - access your personal mails and social networks via telephone and satellite!
- <strong>Hyped by Buzzwords</strong> - certified to run in any container, PaaS, IaaS, *aaS.
- <strong>Lightweight</strong> - uses as little as 64MB of memory and 16MB of disk.
- <strong>Portable</strong> - runs on any flavour of Linux, Unix, and Windows.
- <strong>Independent</strong> - reliably operates without needing additional software or libraries.
- laitos was called ["websh"](https://github.com/HouzuoGuo/websh) before (12+1)th March 2017. 

## Build & Enjoy
Check out the source code under your `$GOPATH` directory and run `go build`.

The friendly [Configuration](https://github.com/HouzuoGuo/laitos/wiki/Configuration) page will then get you started.

## Features (TODO: move this away from homepage)

### For Fun
- Decrypt AES-encrypted files and search for specific lines among them.
- Inspect program environment such as public IP address, memory usage, latest log entries, and stack traces.
- Post updates to Facebook.
- List and read mails from personal mailboxes via IMAPS.
- Send mails to friends.
- Run operating system commands (shell commands).
- Send text message to friend's mobile number.
- Call friend's mobile number and speak a short message.
- Read Twitter home time-line.
- Post updates to Twitter.
- Ask about weather and all sorts of questions on WolframAlpha.

### Daemon services
- Web server
  * Host static HTML home pages (index pages).
  * Host directory of static HTML files and other assets.
- More web services
  * Retrieve environment information (IP address, memory usage, log entries, etc).
  * Use fun features in an interactive web form.
  * Browse and download files from personal Gitlab projects.
  * Web proxy for viewing simple web pages.
  * API hook for Twilio telephone call and Twilio SMS.
- Periodic health check
  * Check that features' API credentials are working.
  * Check that daemon ports are still listened on.
  * Send health reports at regular interval via Email.
- DNS server
  * Automatically tracks advertising domains list.
  * Block ads by refusing to answer to their names.
  * Forward other queries to a well-known DNS server of your choice.
- Mail server
  * Support TLS for communication secrecy.
  * Forward arriving Emails to your personal addresses.
- Telegram bot
  * Use fun features in an interactive chat.
  * Retrieve daemon health information.

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