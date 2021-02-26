## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the endpoint dumps the
incoming HTTP request for your inspection, this includes the request headers, request body, etc.

This often comes in handy as a diagnosis tool for deploying laitos web server behind intermediary components, such as
a load balancer or an API gateway, which often manipulates client's HTTP requests by changing the headers and/or
request body.

## Configuration
Under the JSON key `HTTPHandlers`, add a string property called `RequestInspectorEndpoint`, value being the URL location of the service.

Keep the location a secret to yourself and make it difficult to guess. Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "RequestInspectorEndpoint": "/my-request-inspector",

        ...
    },

    ...
}
</pre>

## Run
The service is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
Use an HTTP client (browser application, command line tool, programming library, etc) to send an HTTP request to the endpoint.

The endpoint will respond with a plain text document, with the following details:

- HTTP request protocol, URL path, query parameters.
- Request headers.
- Request body.

## Tips
- Make the endpoint difficult to guess, this helps to prevent misuse of the service.
