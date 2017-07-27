<img src="https://raw.githubusercontent.com/HouzuoGuo/laitos/master/cosmetic/poster.png" alt="poster image" align="right" />

[![Build Status](https://travis-ci.org/HouzuoGuo/laitos.svg?branch=master)](https://travis-ci.org/HouzuoGuo/laitos)

# laitos - Empower your personal web server
Want to host a simple website, Email server, or ad-blocking DNS server? Skip those scary tutorials you find online!

<strong>laitos</strong> has all your needs covered - plus, it offers you access to personal mails and social networks in fun ways such as telephone call, SMS, Telegram chat, and even satellite terminals!

## Highlights

- <strong>Powerful</strong> - web, mail and DNS servers beautifully re-invented in just 10K lines of code.
- <strong>Fun</strong> - access personal mails and social networks via telephone, SMS, and more!
- <strong>Hyped by Buzzwords</strong> - certified to run in any container, PaaS, IaaS, *aaS.
- <strong>Lightweight</strong> - uses as little as 14MB of memory and 8MB of disk.
- <strong>Portable</strong> - runs on any flavour of Linux, Unix, and Windows.
- <strong>Independent</strong> - reliably operates without additional software or libraries.

<strong>Did you know?</strong> laitos has a cool component that gets you modern & rich web experience on nostalgic computers that run Windows 98!
Explore more in the comprehensive [Feature List](https://github.com/HouzuoGuo/laitos/wiki/Feature-List).

## Get Started
Download ready-to-use [latest release](https://github.com/HouzuoGuo/laitos/releases).
The friendly [Configuration](https://github.com/HouzuoGuo/laitos/wiki/Configuration) page will guide you to craft your own server.

For advanced usage, there are [tips](https://github.com/HouzuoGuo/laitos/wiki/Deployment) for deploying on Amazon/Azure/Google cloud.
To build from source code, run `go get github.com/HouzuoGuo/laitos` and then `go build`.

## Features (TODO: move this away from homepage)

### Fun features for telephone/SMS/telegram and more
- Decrypt AES-encrypted files and search for keywords among the content.
- Retrieve environment information such as IP address, memory usage, log entries, and more.
- Post updates to Facebook.
- List and read mails from personal mailboxes via IMAP.
- Send mails to friends.
- Run operating system commands (shell commands).
- Send text message to friend's mobile number.
- Call friend's mobile number to speak a short message.
- Read Twitter home time-line.
- Post updates to Twitter.
- Ask about weather and all sorts of questions on WolframAlpha.

### Web services
- DNS server
  * Automatically updates advertising domains list.
  * Block ads by refusing to answer to their domain names.
  * Forward other queries to a well-known DNS server of your choice.
- Mail server
  * Support TLS for communication secrecy.
  * Forward arriving Emails to your personal addresses.
- Telegram bot
  * Use fun features in an interactive chat.
  * Retrieve daemon health information.
- Web server
  * Host static HTML pages such as home page.
  * Host file directories.
- More web services
  * Use fun features in an interactive web form.
  * Retrieve environment information such as IP address, memory usage, log entries and more.
  * Browse and download files from personal Gitlab projects.
  * Browse websites via server-side renderer (browser-in-browser).
  * Visit simple websites via a web proxy.
  * API hook for Twilio telephone call and Twilio SMS.
- Periodic health check
  * Validate API credentials used by social networks.
  * Verify that servers are still healthy and running.
  * Send health reports at regular interval via Email.

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

The Golang mascot "gopher" is designed by [Renee French](http://reneefrench.blogspot.com). The gopher side portrait is designed by [Takuya Ueda](https://twitter.com/tenntenn), licensed under the "Creative Commons Attribution 3.0" license.
