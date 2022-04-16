# Introduction

Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server),
the service works as an [HTTP integration web hook](https://www.thethingsindustries.com/docs/integrations/webhooks/)
for [The Things Network](https://www.thethingsnetwork.org/) - a popular IoT
network that uses LoRaWAN for its connectivity.

Be aware of the confusing branding message from "The Things Network" (TTN), it
is also [known as](https://github.com/TheThingsNetwork/docs/issues/473):

- The Things Network Stack
- The Things Network v3
- The Things Stack
- The Things Stack Community Edition
- The Things Stack v3

The HTTP web hook processes three different kinds of incoming messages (aka
"uplink messages") from devices connected to The Things Network (TTN):

- On message port number 129, the web hook picks up a text message from the
  uplink message and stores it in the incoming direction of the message bank of
  "LoRaWAN". If there is a new message (entered within the last 5 minutes) in
  the outgoing direction of the message bank, then the web hook will invoke TTN
  downlink API to transmit the outgoing message back to the device.
- On message port number 112, the web hook picks up an app command completed
  with password PIN from the uplink message, executes the app command using the
  [phone home telemetry handler](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-phone-home-telemetry-handler),
  and then invoke TTN downlink API to transmit the command execution result back
  to the device.

Regardless of the message port number, the web hook always stores the location
(if any), RF signal strength, and payload text of the uplink messages in the
[phone home telemetry handler](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-phone-home-telemetry-handler).

The web hook is generally compatible with any IoT device firmware that sends and
receives compatible payload, though payload expectations are closely aligned with
[hzgl-lora-communicator](https://github.com/HouzuoGuo/hzgl-lora-communicator),
which is an open source IoT firmware for a battery-powered tracker and two-way
messaging device.

The rest of this article may contain instructions specific to
[hzgl-lora-communicator](https://github.com/HouzuoGuo/hzgl-lora-communicator).

## Preparation

1. Sign up for an account at [The Things Network (TTN)](https://www.thethingsnetwork.org/).
2. Login to [The Things Network console](https://console.thethingsnetwork.org/)
   and create a new "application".
3. Follow [hzgl-lora-communicator Get Started](https://github.com/HouzuoGuo/hzgl-lora-communicator/wiki/Get-started)
   to configure "application payload formatter", create a new "end device",
   configure radio parameters for the newly created device
4. Continue following the instructions to compile and flash/upload the firmware
   onto a TTGO T-Beam v1.1 development board.

## Configuration

Under JSON key `HTTPHandlers`, write two string properties called
`LoraWANWebhookEndpoint` and `MessageBankEndpoint`. Make them difficult to guess.

Here is an example setup:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "LoraWANWebhookEndpoint": "/hard-to-guess-ttn-hook",
        "MessageBankEndpoint": "/hard-to-guess-message-ui"

        ...
    },
</pre>

The `LoraWANWebhookEndpoint` will be later used on The Things Network console,
and `MessageBankEndpoint` will serving an HTML form for reading text messages
from IoT devices and entering new messages to be sent to them.

### Optional: allow IoT devices to send in app commands

In addition to `LoraWANWebhookEndpoint` and `MessageBankEndpoint`,follow
[command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor)
to construct a configuration for JSON key `MessageProcessorFilters`.

<pre>
{
    ...
    "MessageProcessorFilters": {
        "PINAndShortcuts": {
            "Passwords": ["verysecretpassword"],
            "Shortcuts": {
                "watsup": ".eruntime",
                "EmergencyStop": ".estop",
                "EmergencyLock": ".elock"
            }
        },
        "TranslateSequences": {
            "Sequences": [
                ["#/", "|"]
            ]
        },
        "LintText": {
            "CompressSpaces": true,
            "CompressToSingleLine": true,
            "KeepVisible7BitCharOnly": true,
            "MaxLength": 160,
            "TrimSpaces": true
        },
        "NotifyViaEmail": {
            "Recipients": ["me@example.com"]
        }
    },
    ...
}
</pre>

The message processor is responsible for executing app commands coming in from
IoT devices.

It may also be useful to enable and configure the [read telemetry record](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-read-telemetry-records)
web service, to read the location (if available), RF signal strength, and
payload data sent by the IoT devices.

## Run

The web hook is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage

Create a new API key for the web hook to send downlink messages to your IoT
devices:

1. Return to TTN console, navigate to the newly created application and then
   visit "API keys".
2. Click "Add API key" and name the new key however you wish.
3. Choose "Grant all current and future rights"
   * If you wish, restrict the privileges of the API key to only those related
     to downlink messages.
4. Click "Create API key" to finish creating the key, copy down the API key into
   a note pad.

Next, create a new web hook for the IoT "application":

1. Navigate to the newly created applications and then visit "Integrations",
   "Webhooks".
2. Click "Add webhook" button and choose "Custom webhook", then fill
   in the details:
   * Webhook ID: (name the webhook however you wish)
   * Webhook format: JSON
   * Base URL: https://laitos-server.example.com/hard-to-guess-ttn-hook (see `LoraWANWebhookEndpoint`)
   * Downlink API key: (paste in the key created just now)
   * Enabled messages: tick "Uplink message" (with an empty path).
3. Save the web hook.

### Usage: exchanging text messages with IoT devices

On the IoT device running [hzgl-lora-communicator](https://github.com/HouzuoGuo/hzgl-lora-communicator),
compose a text message using its morse-based input method. The device will then
start transmitting the text message ("uplink") at regular intervals - usually
every 3 minutes.

Users should visit `MessageBankEndpoint` regularly to read the latest text
messages coming from IoT devices. There is no notification mechanism to alert
users of incoming messages.

To send a reply, visit `MessageBankEndpoint` and enter the reply message in the
"outgoing" directory of "LoRaWAN" message bank. When the communicator IoT device
makes its next transmission within 5 minutes of the reply (the timing is
important), the web hook will use the LoRaWAN network downlink API to transmit
the reply message in a downlink (from IoT gateway to IoT device) message.

### Usage: use IoT devices to execute an app command

On the IoT device running [hzgl-lora-communicator](https://github.com/HouzuoGuo/hzgl-lora-communicator),
compose an app command using its morse-based input method. The device will then
start transmitting the app command at regular intervals - usually every 3
minutes.

When the web hook receives an app command (identified by LoRaWAN message port),
it immediately passes the app command to the message processor (configured by
`MessageProcessorFilters`) for execution.

After executing the command, the message processor retains the command response
in-memory; when the IoT device transmits the app command again, the message
processor will automatically de-duplicate the repeated app command and offers
the previous command response to the web hook for a reply. The web hook then
uses the LoRaWAN network downlink API to transmit the command response in a
downlink (from IoT gateway to IoT device) message.

## Tips

- The message bank will only record 99 historical messages in each
  name-direction combination.
- LoRaWAN transmission should be kept rather short, larger payload cannot be
  reliably sent across in either direction. The practical limit is about 100
  characters in either direction.
  * The web hook will truncate text message reply and app command response to
    100 characters maximum before sending them to IoT devices in downlink
    messages.
