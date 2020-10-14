## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the web service offers
a simplified interface for generic HTTP clients (e.g. command line `curl`) to execute app commands.

## Configuration
1. Under JSON key `HTTPHandlers`, write a string property called `AppCommandEndpoint`, value being the URL location
   of the web service. Keep the location a secret to yourself and make it difficult to guess.
2. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `HTTPFilters`.

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "AppCommandEndpoint": "/very-secret-app-command-endpoint",

        ...
    },

    ...
}
</pre>

## Run
The web service is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
Use a web browser or generic HTTP client such as `curl` to contact the web service:

    curl -X POST 'https://laitos-server.example.com/very-secret-app-command-endpoint' -F 'cmd=PasswordPIN.s echo hello'
    curl -X GET 'https://laitos-server.example.com/very-secret-app-command-endpoint?cmd=PasswordPIN.s+echo+hello'

The web service accepts app command from both form submission (`-F`) and query parameter. The HTTP response comes in plain text (`text/plain`), and it
is subjected to the text linting rules defined in laitos configuration `HTTPFilters`.

## Tips
- Make the URL location secure and hard to guess, it helps to secure this web service beyond password protection!
