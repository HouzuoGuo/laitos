# Feature list

The rich set of components not only covers the basic needs of hosting a personal web server,
but also provide advanced capabilities to satisfy the geeky nature inside of you!

laitos components go into two categories:
- Features - access to Email, post to Twitter/Facebook, etc.
- Daemons - web server, mail server, and chat bots. Daemons grant access to all features.

TODO: Make some screenshots.

## Daemons

### DNS server
DNS server automatically retrieves ad-domain list, and blocks the domains for an ad-free web experience.
It supports DNS-over-TCP as well as UDP.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-DNS-server)

### Mail server
Mail server forwards arriving mails to your personal Email address. It supports TLS certificate for communication secrecy.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-mail-server)

### Telegram messenger chat-bot
Chat-bot provides access to all features via secure infrastructure provided by Telegram Messenger LLP.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-telegram-chat-bot)

### Plain-text sockets
The socket servers provide unencrypted access to all features via TCP and UDP that are accessible via basic tools such as `telnet`, `netcat`, and `HyperTerminal`.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-plain-text-sockets)

### System maintenance
Periodic maintenance patches the system for security updates, and checks for environment and program health.

The results are presented in a report sent to your Email address.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-system-maintenance)

### Web server
Web server serves static HTML file for a home page, and file directories (e.g. HTML/CSS) for a rich personal website.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server)

### Web service - GitLab browser
GitLab browser lists project files and downloads them by specifying file path.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-GitLab-browser)

### Web service - invoke features in a form
The form offers access to all features.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-features-form)

### Web service - program health report
Gather program environment information and conduct a comprehensive program health check, the results are presented in this report.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-health-report)

### Web service - simple proxy
The basic proxy let the web server download web pages for your browser. It however does not provide additional security or anonymity.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-simple-proxy)

### Web service - browser-in-browser
The browser renders web sites on the server and sends back screenshots, enabling you to browse modern Internet using nostalgic technologies such as IE 5 on Windows 98.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-browser-in-browser)

### Web service - Twilio telephone/SMS hook
Triggered by Twilio (communication APIs for SMS, voice, and more), the web hooks enable access to all features via ordinary telephone, SMS, and even satellite terminals.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-telephone-and-SMS-hook-with-Twilio)

## Features

### Social network - Facebook
Post updates to Facebook time-line.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-social-network-Facebook)

### Social network - Twitter
Read and post tweets.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-social-network-Twitter)

### Social network - emails
Send mails to friends, and read mails from personal Email addresses.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-social-network-emails)

### Telephone and SMS
Send text to friend's phone number, or call a friend to speak a short message.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-telephone-and-SMS)

### Utility - two factor authentication code generator
Generate two-factor authentication code for secure website login.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-utility-two-factor-authentication-code-generator)

### Utility - interactive web browser
Browse modern websites interactively in a command-and-answer routine.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-interactive-web-browser)

### Utility - WolframAlpha
Ask about weather and all sorts of questions on WolframAlpha - the computational knowledge engine.

[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-utility-WolframAlpha)

### Utility - find text in AES-encrypted files
Decrypt AES-encrypted files (e.g. password book) and search for keywords among the content.
[Configuration and usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-utility-find-text-in-AES-encrypted-files)

### System operation - run commands
Run operating system commands (shell commands) on Linux and Unix operating systems.

This feature is always available and does not require configuration.

[Usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-system-operation-run-commands)

### System operation - inspect environment
Retrieve server environment information such as IP address, memory usage, log entries, and more.

This feature is always available and does not require configuration.

[Usage](https://github.com/HouzuoGuo/laitos/wiki/Feature:-system-operation-inspect-environment)