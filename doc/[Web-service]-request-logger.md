# Introduction

Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server),
the endpoint starts/stops recording of all incoming HTTP requests and presents
recorded requests for inspection.

This often comes in handy for web developers developing new web hooks (aka "web
callbacks") who may wish to record and inspect incoming web hook requests.

## Configuration

Under the JSON key `HTTPHandlers`, add a string property called
`LatestRequestsInspectorEndpoint`, value being the URL location of the service.

Keep the location a secret to yourself and make it difficult to guess. Here is
an example:

<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "LatestRequestsInspectorEndpoint": "/my-request-recorder",

        ...
    },

    ...
}
</pre>

## Run

The service is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage

Use an HTTP client (browser application, command line tool, programming library,
etc) to send an HTTP request to the endpoint.

To start recording incoming requests, visit the endpoint in a web browser (or
use a command-line HTTP client) with a query parameter `?e=1`, for example
`https://laitos-server.example.com/my-request-recorder?e=1`.

The web server (both `httpd` and `insecurehttpd`) will then start recording all
incoming requests - including the client IP, HTTP request headers, and HTTP
request body.

To inspect recorded requests, visit the endpoint without any query parameters
(`https://laitos-server.example.com/my-request-recorder`).

To stop recording requests, visit the endpoint with a query parameter `?e=0`
(`https://laitos-server.example.com/my-request-recorder?e=0`).

## Tips

- Make the endpoint difficult to guess, this helps to prevent misuse of the
  service.
- The recorder will memorise up to 200 requests. Upon reaching the limit the
  oldest recorded requests will be automatically forgotten to make room for new
  incoming requests.
