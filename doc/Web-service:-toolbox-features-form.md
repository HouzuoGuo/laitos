# Web service: toolbox features form

## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server), the form offers a command
entry text box, through which you may run any toolbox feature.

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

        "CommandFormEndpoint": "/very-secret-toolbox-features-form",

        ...
    },

    ...
}
</pre>

## Run
The form is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server#run).

## Usage
In a web browser, navigate to `CommandFormEndpoint` of laitos web server.

Enter PIN and toolbox command into the text box, click "Exec" button and observe the toolbox command output.

## Tips
Make sure to choose a very secure URL for the endpoint, it is the only way to secure this web service!