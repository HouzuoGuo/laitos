## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the text report is generated
on-demand to show:
- System status:
  * Clock time, uptime.
  * Load and memory usage.
- Program status:
  * Public IP address, uptime.
  * Daemon usage statistics.
- Latest log entries and stack traces.

## Configuration
Under JSON key `HTTPHandlers`, write a string property called `InformationEndpoint`, value being the URL location that
will serve the report. Keep the location a secret to yourself and make it difficult to guess.

Here is an example setup:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "InformationEndpoint": "/very-secret-program-health-report",

        ...
    },

    ...
}
</pre>

## Run
The report is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
In a web browser, navigate to `InformationEndpoint` of laitos web server, and inspect the produced health report.

## Tips
- Make the endpoint difficult to guess, this helps to prevent misuse of the service.

- About the latest log entries presented for inspection:
  * laitos keeps the most recent log entries and warning log entries in memory, totalling several hundreds entries. They
    are available for inspection on-demand. The host operating system or hosting platform may have held more log entries
    available for your inspection.
  * The warning log entries keep track of the most recent (about three dozens) repeating offenders and will not present
    their repeated offences for inspection. For example, when laitos server refuses 30 DNS clients from querying the server
    yet they keep on going, their IPs will only show up once in the recent warnings, until the server refuses another 30,
    different DNS clients from querying the server.
