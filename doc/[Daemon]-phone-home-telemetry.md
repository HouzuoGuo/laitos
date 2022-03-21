# Introduction

The phone home daemon collects system resource usage information and delivers them to your laitos servers via the
[simple app command execution API](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-simple-app-command-execution-API)
and/or [DNS daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server) running on those servers.

You may also ask laitos servers to memorise an app command for this phone home daemon to execute, and view the app
execution result on the laitos servers along with telemetry records from this phone home daemon.

In this article, the term "monitored subject" refers to the telemetry record sender - the one constructing app command
in order to send a telemetry record with sender's system information; on the other hand "laitos server" refers to the
server host of laitos software, the one running daemon programs (web server, DNS server, etc) capable of executing app
commands, and stores received telemetry records in memory.

## Configuration

Construct the following JSON object and place it under key `PhoneHomeDaemon` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>ReportIntervalSec</td>
    <td>integer</td>
    <td>The interval (in seconds) at which the daemon will send telemetry records to all servers.</td>
    <td>300 - all servers get a record every 5 minutes</td>
</tr>
<tr>
    <td>MessageProcessorServers</td>
    <td>Object array, see next table for object properties.</td>
    <td>Details for making contact with your laitos servers.</td>
    <td>This is a mandatory property without a default value.</td>
</tr>
</table>

The `MessageProcessorServers` array contains details of your laitos server that are receiving telemetry records.

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>HTTPEndpointURL</td>
    <td>string</td>
    <td>The URL of your laitos web server's app command execution API endpoint.</td>
    <td>Either this or DNSDomainName must be present in this configuration object.</td>
</tr>
<tr>
    <td>DNSDomainName</td>
    <td>string</td>
    <td>The domain name of your laitos DNS server that is capable of executing app commands.</td>
    <td>Either this or HTTPEndpointURL must be present in this configuration object.</td>
</tr>
<tr>
    <td>Passwords</td>
    <td>array of string</td>
    <td>
      Any one (or more) passwords accepted by your laitos web and DNS servers for authorising app command execution.
      <br />
      Telemetry records are sent by executing app commands on laitos server.
    </td>
    <td>This is a mandatory property without a default value.</td>
</tr>
</table>

The message processor servers may memorise app commands and execute them on this
phone home daemon when the daemon sends telemetry records to the servers. This
enables the message processor servers to remotely control the computer that
runs phone home daemon when needed. To enable this optional feature, follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor)
to construct configuration for configuration JSON key `PhoneHomeFilters`.

Here is a complete example:

<pre>
{
    ...

    "PhoneHomeDaemon": {
        "ReportIntervalSec": 300,
        "MessageProcessorServers": [
            {
                "HTTPEndpointURL": "https://laitos-server-example.com/very-secret-app-command-endpoint"
                "Passwords": ["MyHTTPFiltersPasswordPIN"]
            },
            {
                "DNSDomainName": "laitos-server-example.com"
                "Passwords": ["MyDNSFiltersPasswordPIN"]
            }
        ]
    },
    "PhoneHomeFilters": {
        "PINAndShortcuts": {
            "Passwords": ["PhoneHomePassword"],
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
            "CompressSpaces": false,
            "CompressToSingleLine": false,
            "KeepVisible7BitCharOnly": false,
            "MaxLength": 4096,
            "TrimSpaces": false
        },
        "NotifyViaEmail": {
            "Recipients": ["me@example.com"]
        }
    },

    ...
}
</pre>

## Run

Tell laitos to run the phone home daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,phonehome,...

## Usage

The phone home daemon automatically sends telemetry records consisting of host name, platform information (CPU, OS),
and system resource usage (memory & disk) to your laitos servers.

Instead of sending telemetry records to all of the servers at the same time, the daemon divides the reporting interval
by the number of servers, and sends a telemetry record to one at a time at the divided interval. For example, if
report interval is 300 seconds and there are 10 servers, the daemon will shuffle the server list randomly, send a telemetry
record to the first server, wait for 30 seconds, send to the second server, and so on.

Use web service [read telemetry records](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-read-telemetry-records)
to read the telemetry records sent by this daemon. A record looks like:

<pre>
{
    "OriginalRequest": {
        "SubjectIP": "123.123.123.123",
        "SubjectHostName": "my-laptop",
        "SubjectPlatform": "linux-amd64",
        "SubjectComment": {
            "CLIFlags": [
                ...
                "-supervisor=false",
                "-daemons",
                "autounlock,maintenance,phonehome"
            ],
            "ClockTime": "2021-12-14T18:58:36.198344935Z",
            "DiskCapMB": 15817,
            "DiskFreeMB": 2845,
            "DiskUsedMB": 12972,
            "EGID": 0,
            "EUID": 0,
            "EnvironmentVars": [
                ...
                "SHELL=/bin/sh",
                "HOME=/root",
                "LANG=C.UTF-8",
                ...
            ],
            "ExePath": "/hg/bin/laitos.amd64",
            ...
            "WorkingDirContent": [
                ...
                "index.html",
                "resume/",
                ...
            ],
            "WorkingDirPath": "/prog-dat/"
        },
        "CommandRequest": {
            "Command": ""
        },
        "CommandResponse": {
            "Command": "",
            "ReceivedAt": "0001-01-01T00:00:00Z",
            "Result": "",
            "RunDurationSec": 0
        }
    },
    "SubjectClientID": "123.123.123.123",
    "ServerTime": "2020-07-21T06:09:36.989085597Z",
    "DaemonName": "httpd"
},
</pre>

When the phone home daemon allows message processor servers to execute app
commands on this computer via the phone home daemon (i.e. by configuring
`PhoneHomeFilters`), contact the same web service [read telemetry records](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-read-telemetry-records)
to execute an app command on the phone home daemon's computer:

    curl 'https://laitos-server.example.com/very-secret-telemetry-retrieval?tohost=SubjectHostName&cmd=PhoneHomePassword.s+echo+abc'

When the phone home daemon sends its next telemetry record, it will pick up
the app command from the message processor server's response and execute it.
When the phone home daemon sends yet another telemetry record, the record will
include the command execution result (`CommandResponse`).

## Tips

- The daemon never transmits the password PIN over network, instead, it
  translates the password PIN into a disposable, one-time-password with every
  telemetry record. This is especially helpful when sending telemetry over DNS,
  as DNS protocol does not use encryption. Read more about this command
  processor mechanism in [Use one-time-password in place of password](https://github.com/HouzuoGuo/laitos/wiki/Command-processor#use-one-time-password-in-place-of-password).
- When the daemon sends out a telemetry record over DNS to your laitos server,
  the record will appear truncated on the receiver's end. This is to be expected
  as DNS protocol does not leave much room for data transmission.
- In an outgoing telemetry record, the host name is always truncated to 16
  characters maximum and changed to lower case. This is especially beneficial
  for sending the telemetry record over DNS which has very limited space for
  data transmission.
- The [DNS daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server)
  automatically allows telemetry record senders to send DNS queries as well,
  regardless of whether the sender's IP is among the `AllowQueryIPPrefixes`.
  This is a handy alternative to keeping `AllowQueryIPPrefixes` updated for the
  public IP of your home network.
