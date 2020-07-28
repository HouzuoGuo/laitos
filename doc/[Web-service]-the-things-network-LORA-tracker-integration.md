## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the service
works as an [HTTP integration](https://www.thethingsnetwork.org/docs/applications/http/) method for
[The Things Network](https://www.thethingsnetwork.org/).

When your LoRa IoT devices run a [The Things Network Mapper compatible](https://ttnmapper.org/) program, this
web service collects the devices' location information via The Things Network HTTP integration, and stores each
location record in-memory via the [telemetry handler app](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-phone-home-telemetry-handler),
making them available for inspection via the [read telemetry records web service](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-read-telemetry-records).

Optionally, you may extend the Mapper IoT program to include an app command with each transmission, when the web
service receives the app command along with IoT device location from "uplink" transmission, it will execute the
app command and put the command response in a "downlink" message for the IoT device to receive.

## Preparation
1. Sign up for an account at [The Things Network (TTN)](https://www.thethingsnetwork.org/). The Things Network
   is an open IoT platform that uses the popular [LoRa (LoRaWAN)](https://en.wikipedia.org/wiki/LoRa) for radio
   transmission.
2. Login to [TTN Applications Console](https://console.thethingsnetwork.org/) and create an application. An
   application hands out access key for your LoRa IoT devices that share the same program, same data format, and
   same set of Internet integration.
3. Install a [TTN Mapper](https://ttnmapper.org/) compatible program to your LoRa IoT device, such as this program
   made for TTGO T-Beam development board [ttgo-tbeam-ttn-tracker](https://github.com/kizniche/ttgo-tbeam-ttn-tracker).
4. In TTN applications console, navigate to "Devices" and register your IoT development device there, the registration
   grants the device free access to TTN and hands it a set of credentials that must be by the IoT device's program.
5. In TTN applications console, navigate to "Payload Formats", and choose to use a "Custom payload format". The decoder
   interprets individual bytes transmitted by IoT device and transforms them into an enriched data record, write down
   the following code for the decoder:

       // from https://github.com/kizniche/ttgo-tbeam-ttn-tracker
       function Decoder(bytes, port) {
         var decoded = {};
         decoded.latitude = ((bytes[0]<<16)>>>0) + ((bytes[1]<<8)>>>0) + bytes[2];
         decoded.latitude = (decoded.latitude / 16777215.0 * 180) - 90;
         decoded.longitude = ((bytes[3]<<16)>>>0) + ((bytes[4]<<8)>>>0) + bytes[5];
         decoded.longitude = (decoded.longitude / 16777215.0 * 360) - 180;
         var altValue = ((bytes[6]<<8)>>>0) + bytes[7];
         var sign = bytes[6] & (1 << 7);
         if(sign) decoded.altitude = 0xFFFF0000 | altValue;
         else decoded.altitude = altValue;
         decoded.hdop = bytes[8] / 10.0;
         decoded.sats = bytes[9];
         return decoded;
       }

Optionally, if you with to contribute to the public of world-wide TTN coverage, consider making your IoT device location
records public by visiting TTN application console, navigate to "Integrations", and add "TTN Mapper" integration.

## Configuration
Under JSON key `HTTPHandlers`, write a string property called `TheThingsNetworkEndpoint`, value being the URL address
of the service. The address should be kept a secret, make it difficult to guess.

If you wish to extend the TTN Mapper-compatible program and enable your IOT device to execute app commands on this laitos
server, then follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration
for JSON key `HTTPFilters`.

Here is an example setup:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "TheThingsNetworkEndpoint": "/very-secret-ttn-integration",

        ...
    },

    ...

    "HTTPFilters": {
        "PINAndShortcuts": {
            "PIN": "verysecretpassword",
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

In addition, please enable and configure the [read telemetry record](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-read-telemetry-records)
web service, in order to read the location data sent by your IoT devices.

## Run
The service is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
1. In TTN applications console, navigate to "Integrations", add a new HTTP integration. Give the integration an identifier name
   (so called Process ID), choose the default key for Access Key, and use the laitos web service URL there; use "POST" as the
   HTTP method, and leave Authorization and custom header blank.
2. Power up your LoRa IoT device(s). In TTN application console, inspect the status of your LoRa IoT device registration, and
   verify that it is indeed transmitting location data properly.
3. In web browser, Navigate to laitos web service [read telemetry record](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-read-telemetry-records)
   to read the location records from your IoT devices.

If you wish to send app commands from LoRa IoT devices to your laitos server, modify the TTN mapper program running on the IoT
devices in this way:

1. Leave the transmission of TTN mapper-compatible location payload intact.
2. Write additional program logic to append app command, completed with password PIN, to the transmitted payload. The original
   TTN mapper program transmits location data in exactly 10 bytes, so the app command should start at the 11th byte.
3. In TTN applications console, leave the decoder program intact - the present decoder program continues to successfully decode 
   location data, the bytes added to payload do not interfere with the decoder.
3. This web service, upon receiving the transmitted app command, will pick up the app command in addition to location data, and
   execute the app command using command processor configuration from `HTTPFilters`.
4. After laitos executes the app command, the web service will ask TTN to transmit the command response to your LoRa IoT devices,
   TTN calls this "scheduling a downlink message", which will be picked up by your LoRa IoT devices when it makes the next radio
   transmission.

Be aware that, sending downlink message (the app command reply) is the latest feature of The Things Network, its reliability leaves
a great deal to be desired, and the downlink coverage is far less than the uplink message.

## Tips
For transmitting app commands from IoT device to laitos server:
- The web service picks up app commands transmitted by your IoT devices from the transmission's raw bytes, which is why the TTN
  decoder program needs not to be modified to support app command transmission.
- It is OK to repeatedly transmit the same app command, doing so in fact ensures a higher likelihood of successful delivery.
  The mechanism that prevents execution of repeated app commands is the same one that powers [phone home telemetry handler](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-phone-home-telemetry-handler)
