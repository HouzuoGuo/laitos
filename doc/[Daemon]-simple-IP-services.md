## Introduction
The simple IP services implement standard Internet services that were used in the nostalgic era of computing.

The three services are:
- Active system user names (sysstat) - [rfc866](https://tools.ietf.org/html/rfc866)
- Date and time (daytime) - [rfc867](https://tools.ietf.org/html/rfc867)
- quote of the day (QOTD) - [rfc865](https://tools.ietf.org/html/rfc865)

## Configuration
Construct the following JSON object and place it under key `SimpleIPSvcDaemon` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The address network to listen on.</td>
    <td>"0.0.0.0" - listen on all network interfaces.</td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>Maximum number of requests a client (identified by IP) may make in a second.</td>
    <td>6 - good enough for most cases</td>
</tr>
<tr>
    <td>ActiveUsersPort</td>
    <td>integer</td>
    <td>TCP and UDP port number to listen on for "sysstat" (active users) service.</td>
    <td>11 - the well-known port number designated for the service.</td>
</tr>
<tr>
    <td>ActiveUserNames</td>
    <td>string</td>
    <td>A single line of text to respond to "sysstat" service clients.</td>
    <td>Empty string</td>
</tr>
<tr>
    <td>DayTimePort</td>
    <td>integer</td>
    <td>TCP and UDP port number to listen on for "daytime" service.</td>
    <td>13 - the well-known port number designated for the service.</td>
</tr>
<tr>
    <td>QOTDPort</td>
    <td>integer</td>
    <td>TCP and UDP port number to listen on for "QOTD" service.</td>
    <td>17 - the well-known port number designated for the service.</td>
</tr>
<tr>
    <td>QOTD</td>
    <td>string</td>
    <td>A single line of text to respond to "QOTD" service clients.</td>
    <td>Empty string</td>
</tr>
</table>

Here is a minimal setup example:

<pre>
{
    ...

    "SimpleIPSvcDaemon": {
        "ActiveUserNames": "matti",
        "QOTD": "cheese cake is delicious"
    },

    ...
}
</pre>

## Run
Tell laitos to run the daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,simpleipsvcd,...

## Usage
Contact the three services via either TCP or UDP, for example via the `netcat` command:

    > nc localhost 11
    matti
    ^C
    > $ nc localhost 13
    2019-02-25T17:25:34Z
    ^C
    > nc localhost 17
    cheese cake is delicious
    ^C

Keep in mind that UDP behaves differently - the client needs to send something before server responds:

    > nc -u localhost 11
    something
    matti
    ^C
    > nc -u localhost 13
    something
    2019-02-25T17:29:14Z
    ^C
    > nc -u localhost 17
    somethjing
    cheese cake is delicious
    ^C
