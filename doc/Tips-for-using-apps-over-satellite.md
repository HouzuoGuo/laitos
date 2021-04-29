# Tips for using apps over satellite

The laitos software helps you to use Internet features (such as browsing news
and social network) while taking advantage of the extended communication range
offered by satellite terminals.

Inmarsat and Iridium are two major satellite service providers that offer
commercial grade satellite handsets capable of making and receiving phone calls,
sending and receiving plain-text SMS and Emails, as well as using IP-based data
at a very limited 2400 bits-per-second. The laitos software has been extensively
tested on their handsets.

There are several ways to reach laitos and run app commands from a satellite
phones.

## By dialing a phone number

First, purchase a phone number from Twilio and setup laitos
[Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook),

On the satellite handset, dial the phone number, and use the
[DTMF input technique](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook#usage)
to enter an app command using number pad, and then press hash key to indicate
the end of app command input.

Then laitos server will run the app command and speak the command response back
to the caller.

Though a phone number from Twilio can receive SMS from a satellite phone, the
number cannot send an SMS reply to the phone, making it difficult to use laitos
apps over SMS.

## By exchanging Emails

First, setup laitos
[mail server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-mail-server)
to let laitos server receive Emails.

On the satellite handset, compose a text message consisting of your app command
password PIN followed by the app command to run, and send it to laitos server
over Email.

The laitos server will run the app command and send the command response back to
the sender in an Email reply. Be aware that satellite service providers often
truncate this Email reply to ~140 characters in length.

## By using narrow band IP data

The satellite handsets offer very limited IP-data capability by encapsulating
data packets over what is essentially a voice circuit, similar to an old-school
modem, resulting in very limited bandwidth of ~2400 bits/second. General web
browsing cannot work over such a narrow bandwidth.

Because laitos app commands and responses are extremely compact, they work
surprisingly well - there are several ways to reach laitos:

-   By using the
    [laitos terminal interface](https://github.com/HouzuoGuo/laitos/wiki/Laitos-terminal).
    Make sure to turn on the "low bandwidth mode", and the terminal will use
    HTTP (instead of HTTPS) to reach laitos server.
    *   2400 bits/second bandwidth is insufficient for TLS handshake used by
        HTTPS.
-   By connecting to
    [laitos telnet server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-telnet-server).
-   By connecting to
    [laitos web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server)
    over HTTP and use the
    [app command form](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-invoke-app-command)
    or
    [simple app command execution API](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-simple-app-command-execution-API).
