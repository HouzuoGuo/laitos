## Introduction

In addition to providing your home network a safer and cleaner web experience,
the [DNS server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server)
is also capable of invoking app commands from `TXT` queries.

This enables using the entire suite of [laitos apps](https://github.com/HouzuoGuo/laitos/wiki/Component-list#apps)
(reading the news, checking emails, etc) in a restrictive network where normal
TCP/IP communication is unavailable - which is rather common with in-flight WiFi
and hotspots with captive portals.

## Configuration

First, add the [DNS server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server)
daemon configuration (`DNSDaemon`) to the JSON config file.

Make sure to specify laitos server(s)' own domain or sub-domain names in
`MyDomainNames` - laitos will handle `TXT` queries for tehse domain names on its
own, without forwarding them.

Then construct an [app command processr](https://github.com/HouzuoGuo/laitos/wiki/Command-processor)
under key `DNSFilters` in the JSON config file.

Here is a complete example:

<pre>
{
    ...

    "DNSDaemon": {
        "AllowQueryFromCidrs": ["35.196.0.0/16", "37.228.0.0/16"],
        "MyDomainNames": ["my-laitos-server.com", "my-laitos-server.net"]
    },
    "DNSFilters": {
        "PINAndShortcuts": {
            "Passwords": ["mypassword"],
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

## Route TXT queries from your domain name to laitos DNS server

### For an apex domain (laitos-example.com)

The laitos DNS server automatically serves a number of records for an APEX
domain - SOA, NS, and address records for the NS.

Visit your domain registrar and configure these name servers for the domain:

- `ns1.laitos-example.com`
- `ns2.laitos-example.com`
- `ns3.laitos-example.com`
- `ns4.laitos-example.com`

Next, configure "glue records" for the name servers - seek information from the
registrar's support if needed. Add glue records for all 4 name servers and point
to your laitos server's public IP.

### For a sub-domain (sub.laitos-example.com)

Create the following records in the parent zone (e.g. `laitos-example.com`):

<table>
    <tr>
        <th>Record</th>
        <th>Type</th>
        <th>Value</th>
    </tr>
    <tr>
        <td>sub.laitos-example.com</td>
        <td>NS</td>
        <td>ns-sub.laitos-example.com</td>
    </tr>
    <tr>
        <td>ns-sub.laitos-example.com</td>
        <td>A</td>
        <td>(your laitos server's public IP)</td>
    </tr>
</table>

## Run

App commands execution is built into the DNS server daemon. Run the DNS daemon
by specifying it in the laitos command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,dnsd,...

## Usage

To invoke an app command, it needs to be transformed into a `TXT` DNS query:

1. Compose the app command, e.g. `mypassword.s echo 123` (run shell command
   `echo 123`).
2. Substitute all numbers and symbols with the [DTMF input sequence table](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook#usage).
   e.g. `mypassword1420s0echo0110120130`.
3. Use the prefix `_` with the app command, e.g.
   `_mypassword1420s0echo0110120130`.
4. If the command is longer than 63 characters, split it into individual
   segments of less than 63 characters each, and concatenate
   the segments with a dot, e.g. `_mypassword.1420s0.echo0110120130`.
5. Append the laitos (sub-)domain name at the end, e.g.
    `_mypassword.1420s0.echo0110120130.sub.laitos-example.com`.

The query is now ready. Send the query using a command line tool such as `dig`:

    > dig -t TXT _mypassword.1420s0.echo0110120130.sub.laitos-example.com +timeout=30
    ; <<>> DiG 9.9.4-RedHat-9.9.4-61.amzn2.1.1 <<>> -t TXT _mypassword.1420s0.echo0110120130.sub.laitos-example.com
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 33180
    ;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 0
    
    ;; QUESTION SECTION:
    ;_mypassword.1420s0.echo0110120130.sub.laitos-example.com. IN TXT
    
    ;; ANSWER SECTION:
    _mypassword.1420s0.echo0110120130.sub.laitos-example.com. 30 IN TXT "123"
    
    ;; Query time: 29 msec
    ;; SERVER: 10.12.0.2#53(10.12.0.2)
    ;; WHEN: Mon Feb 25 18:41:51 UTC 2019
    ;; MSG SIZE  rcvd: 167

The command response goes into the DNS answer section. The answer uses a
time-to-live of 30 seconds, which means repeating the same command within 30
seconds will produce stale result.

## Tips

General tips:

- Respect and comply with the terms and conditions of your Internet service
  and captive portal service providers.
- The entire DNS query - including the app command, the dedicated domain name,
  and dots in between DNS labels, may not exceed 254 characters. The command
  response (answer) will be automatically truncated to a maximum of 254
  characters.

Regarding timing:

- By default, each app command is given 29 seconds to complete unless the
  timeout duration is overridden by `PLT` command processor mechanism.
- When an app command takes longer than ~5 seconds to complete, the recursive
  resolver issuing the query will consider it a timeout (upstream name server
  failure). Do not worry - internally, laitos patiently waits for the app
  command to complete and makes the command response ready for retrieval when
  the user makes the same query within 30 seconds.

Regarding security:

- DNS queries are not encrypted. The app command input is susceptible to
  eavesdropping on the public Internet.
  * Only use DNS for app command invocation as a last resort when all other
    encrypted channels are unavailable.
  * Consider using [one-time-password is place of password](https://github.com/HouzuoGuo/laitos/wiki/Command-processor#use-one-time-password-in-place-of-password).
