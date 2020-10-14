## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the form offers a text field
to submit app commands and displays the response.

## Configuration
1. Under JSON key `HTTPHandlers`, write a string property called `CommandFormEndpoint`, value being the URL location
   that will serve the form. Keep the location a secret to yourself and make it difficult to guess.
2. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `HTTPFilters`.

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "CommandFormEndpoint": "/very-secret-invoke-app-command",

        ...
    },

    ...
}
</pre>

## Run
The form is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
In a web browser, navigate to `CommandFormEndpoint` of laitos web server.

Enter password and app command into the text box, click "Exec" button and observe the app response.

## Tips
- Make the URL location secure and hard to guess, it helps to secure this web service beyond password protection!
