## Introduction
The telemetry handler (code named "store&forward message processor") reads telemetry information fields from input
parameters, and stores them in memory, associated with the host field presented in the input.

A monitored subject capable of sending telemetry information contacts laitos on this app via any of the enabled
laitos daemons.

This app is not used in manual ways, instead:
- On a monitored subject, the [phome home daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry)
  automatically contacts a laitos server collecting telemetry information by invoking this app on that server.
- On the laitos server, [read telemetry records](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-read-telemetry-records)
  is a web service that offers an HTTP interface for reading collected telemetry records, and optionally remembering
  an app command for monitored subject to run.

In this article, the term "monitored subject" refers to the telemetry record sender - the one constructing app command
in order to send a telemetry record with sender's system information; on the other hand "laitos server" refers to the
server host of laitos software, the one running daemon programs (web server, DNS server, etc) capable of executing app
commands, and stores received telemetry records in memory.

## Configuration
Under JSON object `Features`, construct a JSON object called `MessageProcessor` that has the following properties:

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>MaxReportsPerHostName</td>
    <td>positive integer</td>
    <td>Maximum number of records retained in memory for each monitored subject, identified by their self-reported host name.</td>
    <td>864 (enough for 3 days of records at the default interval of phone home daemon)</td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

         "MessageProcessor": {
             "MaxReportsPerHostName": 500
         },

        ...
    },

    ...
}
</pre>

When a monitored subject sends a telemtry information record by contacting laitos server on this app, their telemetry record
may optionally include an app command that they would like this server to run. If you wish to enable this , then follow
[command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for JSON key
`MessageProcessorFilters`, for example:

<pre>
{
    ...

    "`MessageProcessorFilters`": {
        "PINAndShortcuts": {
            "PIN": "MessageProcessorFiltersPasswordPIN",
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
            "MaxLength": 255,
            "TrimSpaces": true
        },
        "NotifyViaEmail": {
            "Recipients": ["me@example.com"]
        }
    },

    ...
}
</pre>

## Usage
This app is not used in manual ways, instead, the [phome home daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry)
constructs a command intended for this app and transmits it automatically.

For programming reference, the app command is invoked this way:

    .0m Field1\x1fField2\x1fField3\x1....

There are 9 fields in total, the fields are separated by the character of ASCII Unit Separator (`\x1f`). The fields are collected from the perspective
of telemetry information sender (the monitored subject), A field without information will be an empty string with the trailing unit separator.

Here are the 9 fields:

1. Host name.
2. An app command that the monitored subject would like laitos server to run (e.g. `MessageProcessorFiltersPasswordPIN .s echo 123`).
3. The app command that the laitos server previously asked the monitored subject to run (e.g. `PhoneHomePasswordPIN.s echo 456`).
4. Monitored subject's response in response to the command from 3rd field (e.g. `456`).
5. Platform name - `GOOS-GOARCH` (e.g. `linux-amd64`).
6. Comment - program status, system load and memory usage, etc. This comment text is identical to the output of `.e info` from
   [program control app](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-inspect-and-control-server-environment).
7. Public IP address.
8. The Unix timestamp (in second) at which the monitored subject received the app command from the 3rd field.
9. The duration (in seconds) it took for the monitored subject to execute the app command from the 3rd field.

If due to memory/protocol constraints a monitored subject cannot transmit all 9 fields, it is OK for it to omit any number of the rightmost fields.
In fact the first field (host name) is the only mandatory field. The fields are intentionally ordered from most important to least important.

The app response comes in a JSON string:

<pre>
{
    "CommandRequest": {
        "Command": "PhoneHomePasswordPIN.s echo 456"                # laitos server would like monitored subject to run this app command
    },
    "CommandResponse": {
        "Command": "MessageProcessorFiltersPasswordPIN.s echo 123", # monitored subject previously asked laitos server to run this app command
        "ReceivedAt": 1234567,                                      # unix timestamp at which laitos server received the app command
        "Result": "123",                                            # app command execution result
        "RunDurationSec": 3                                         # the duration it took for the app command to execute
    }
}
</pre>

Upon receiving the app response in JSON, the phone home daemon will log the command response and honor the command request.

## Tips
- If a monitored subject is not heard from for 3 consecutive days, it will be removed (cleaned up) from memory.
- The app tightly integrates with the [phone home daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry), working together
  they allow monitored subjects and laitos server to execute custom app commands on each other - with a high degree of reliability. The mechanism codenamed
  "store&forward message processor" allows either party to repeatedly send identical command to the other party, to ensure a very high likelihood of
  successful delivery.
- When the app and phone home daemon (if used) ask each other to run custom app commands, each of them will retain their most recent custom app command and
  response in-memory for up to 3000 seconds. If a custom app command duplicates that which was previously run, the duplicated app command will be ignored.
  The JSON response will then return the custom command and response ran previously. The retained recent app command and responses are automatically
  cleared after 3000 seconds.
- In general, the app command processor universally used by all laitos apps works in a line-oriented fashion, therefore, if a line break (`\n`) shows
  up in the 4th (app command response) or 6th (comment) field, they must be substituted with ASCII Record Separator (`\x1e`), and laitos will recover
  line breaks from them when decoding the fields.
