# Web service: Twilio telephone/SMS hook

## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server), the web service is triggered
by incoming calls and SMS from Twilio platform, and offers caller/sender access to all toolbox features.

That means: using telephone, SMS, and satellite terminals you will have to personal Emails, Facebook, Twitter, and more!

## Configuration
Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
JSON key `HTTPBridges`. Make sure to limit `MaxLength` of `LintText` to a reasonable number below 1000, otherwise an
unexpectedly large command response may incur high fees.

Then, in order to enable telephone call hook, construct the following properties under JSON key `HTTPHandlers`:
1. A string property called `TwilioCallEndpoint`, value being the URL location that will serve the form. Keep the
   location a secret to yourself and make it difficult to guess.
2. An object called `TwilioCallEndpointConfig` with only a string property `CallGreeting`, value being a greeting
   message spoken to telephone caller.

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "TwilioCallEndpoint": "/very-secret-twilio-call-service",
        "TwilioCallEndpointConfig": {
            "CallGreeting": "Hello from laitos"
        },
        "TwilioSMSEndpoint": "/very-secret-twilio-sms-service",

        ...
    },

    ...
}
</pre>

## Run
The form is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server#run).

## Usage
FIXME: TODO:

## Tips
Make sure to choose a very secure URL for both call and SMS endpoints, it is the only way to secure this web service!