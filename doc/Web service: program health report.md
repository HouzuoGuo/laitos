# Web service: program health report

## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server), the text report is generated
on-demand to show:
- System status:
  * Clock time, uptime.
  * Load and memory usage.
- Program status:
  * Public IP address, uptime.
  * Daemon usage statistics.
- Health check - test all API credentials and passwords.
- Latest log entries and stack traces.

## Configuration
Under JSON key `HTTPHandlers`, write a string property called `InformationEndpoint`, value being the URL location that
will serve the report. Keep the location a secret to yourself and make it difficult to guess.

Here is an example setup that hosts a home page and media files:
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
The form is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server#run).

## Usage
In a web browser, navigate to `InformationEndpoint` of laitos web server, and inspect the produced health report.

## Tips
Make sure to choose a very secure URL for the endpoint, it is the only way to secure this web service!

The health report serves very well as a health check URL if a load balancer needs it.

Be aware that expired/incorrect API credentials will still produce a HTTP 200 OK response, because technically it is a
fault of configuration that has to be manually corrected.