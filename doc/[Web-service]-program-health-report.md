## Introduction

Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server),
the comprehensive report shows:

- System status:
  - Clock time, uptime.
  - System load, disk, and memory usage.
- Program status:
  - Public IP address, uptime.
  - Program environment, working directory.
  - Daemon requests statistics.
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

In a web browser, navigate to `InformationEndpoint` of laitos web server, and inspect the program health report in plain text.

Using a programmable HTTP client (e.g. curl), use the `Accept: application/json` to retrieve JSON-formatted response.

## Tips

- Make the endpoint difficult to guess, this helps to prevent misuse of the service.
- laitos only keeps a small number of recent log entries in memory for inspection.
  Seek host OS's help to view more historical log entries.
- Some log messages are automatically discarded due to throttling, see also [High request per second and logging](https://github.com/HouzuoGuo/laitos/wiki/Cloud-tips#high-request-per-second-and-logging)
