## Introduction
The DNS server daemon is a recursive DNS resolver that provides a safer web experience by blocking most of advertisement
and malicious domains.

At start up and then once a day, the blacklists for advertisement and malicious domains are automatically updated from
well-known sources:
- [malwaredomainlist.com](http://www.malwaredomainlist.com)
- [someonewhocares.org](http://someonewhocares.org/hosts/hosts)
- [mvps.org](http://winhelp2002.mvps.org)
- [yoyo.org](http://pgl.yoyo.org)
- [oisd.nl light hosts list](https://oisd.nl/)
- [The Block List Project (ransomware/scam/tracking)](https://github.com/blocklistproject/Lists)

Beyond the blacklists, the DNS resolver uses redundant set of secure and trusted public DNS services provided by:
- [Quad9](https://www.quad9.net)
- [CloudFlare with malware prevention](https://blog.cloudflare.com/introducing-1-1-1-1-for-families/)
- [OpenDNS](https://www.opendns.com)
- [AdGuard DNS](https://adguard.com/en/adguard-dns/overview.html)
- [SafeDNS](https://www.safedns.com)

## Configuration
Construct the following JSON object and place it under key `DNSDaemon` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>AllowQueryIPPrefixes</td>
    <td>array of strings</td>
    <td>
        An array of IP address prefixes such as ["195.1", "123.4.5"] that are allowed to make DNS queries.
        <br/>
        The public IP address of your wireless routers, computers, and phones should be listed here.
    </td>
    <td>(This is a mandatory property without a default value)</td>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The address network to listen on.</td>
    <td>"0.0.0.0" - listen on all network interfaces.</td>
</tr>
<tr>
    <td>Forwarders</td>
    <td>array of "IP:port" strings</td>
    <td>Public DNS resolvers (IP:Port) to use. They must be able to handle both UDP and TCP for queries.</td>
    <td>Quad9, SafeDNS, OpenDNS, AdGuard DNS, Neustar.</td>
</tr>
<tr>
    <td>UDPPort</td>
    <td>integer</td>
    <td>UDP port number to listen on.</td>
    <td>53 - the well-known port number designated for DNS.</td>
</tr>
<tr>
    <td>TCPPort</td>
    <td>integer</td>
    <td>TCP port number to listen on.</td>
    <td>53 - the well-known port designated for DNS.</td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>
        Maximum number of queries a client (identified by IP) may make in a second.
        <br/>
        Each computer/phone usually uses less than 50.
    </td>
    <td>48 - good enough for 3 devices</td>
</tr>
</table>

Here is a minimal setup example:

<pre>
{
    ...

    "DNSDaemon": {
        "AllowQueryIPPrefixes": ["195", "35.196", "35.158.249.12"]
    },

    ...
}
</pre>

## Run
Tell laitos to run DNS daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,dnsd,...

## Test
Assuming that daemon listens on port 53, perform the tests from a computer where you will use the ad-blocking DNS server,
such as home network:

1. Observe successful "Name-Address" answers from the following two system command (for both UDP and TCP):

        nslookup microsoft.com <SERVER PUBLIC IP>
        nslookup -vc microsoft.com <SERVER PUBLIC IP>

2. Observe a black-hole answer `0.0.0.0` from the following query to advertisement domain:

        nslookup analytics.google.com <SERVER PUBLIC IP>
        nslookup -vc analytics.google.com <SERVER PUBLIC IP>

If the test is conducted on the computer that runs daemon itself, you may use `127.0.0.1` as the server IP address.

If the tests are not successful, and laitos log says `client IP is not allowed to query`, then check the value of
`AllowQueryIPPrefix` in configuration.

## Usage
After the DNS server is successfully tested, it is ready to be used by your computers and phones.

On your computers and phones, follow these guides and change DNS settings to use *public IP address* of laitos server:

- Windows/Mac [tutorial by Google](https://developers.google.com/speed/public-dns/docs/using#change_your_dns_servers_settings)
- Alternative Windows [tutorial by windowscentral.com](https://www.windowscentral.com/how-change-your-pcs-dns-settings-windows-10)
- Android [tutorial by OpenDNS](https://support.opendns.com/hc/en-us/articles/228009007-Android-Configuration-instructions-for-OpenDNS)
- iOS [tutorial by igeeksblog.com](https://www.igeeksblog.com/how-to-change-dns-on-iphone-ipad/)

## Tips
Regarding usage:
- Computers and phones usually memorise DNS settings per network, make sure to change DNS settings for all wireless and
  wired networks.
- Use a well-known public DNS service (e.g. Cloudflare DNS, Google DNS) as backup DNS server in DNS settings, so that in
  the unlikely case of laitos DNS server going offline, your computers and phones will still be able to browse the
  Internet.

Regarding configuration:
- Not all DNS services support TCP for queries. The default forwarders (CloudFlare, Quad9, SafeDNS, OpenDNS) support both
  TCP and UDP equally well.
- If given, the DNS `Forwarders` will override all default forwarders, and the default forwarders will remain inactive.

## Invoke app commands via DNS queries
Beside offering an ad-free and safe web experience, the DNS server can also invoke app commands via `TXT` queries, this
enables Internet usage in an environment where DNS usage is unrestricted but Internet access is not available.

### Configuration
To enable app command execution, follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor)
to construct configuration for JSON key `DNSFilters`, for example:

<pre>
{
    ...

    "DNSDaemon": {
        "AllowQueryIPPrefixes": ["195", "35.196", "35.158.249.12"]
    },
    "DNSFilters": {
        "PINAndShortcuts": {
            "PIN": "mypassword",
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

### Prepare domain names
Next, designate two domain names (sub-domains do not help) under your control to participate in the next stage of
setup:
1. The first domain name is a throw-away domain name, it will offer app command execution capability, but will not
   be able to host other services such as web server and mail server.
   As an example: `my-throw-away-domain-example.net`
2. The second domain will provide name server resolution for the first domain name and can host web and mail services as
   usual. As an example: `my-home-domain-example.net`

Prepare between two and four type `A` DNS records on the second domain name, point the records to laitos DNS server,
the record names will become authoritative name server of the first domain name. For example, pretending that
`123.234.123.234` is laitos DNS server, create the following DNS records for `my-home-domain-example.net`:

<table>
    <tr>
        <th>Record</th>
        <th>Type</th>
        <th>Value</th>
    </tr>
    <tr>
        <td>ns1-for-laitos.my-home-domain-example.net</td>
        <td>A</td>
        <td>123.234.123.234</td>
    </tr>
    <tr>
        <td>ns2-for-laitos.my-home-domain-example.net</td>
        <td>A</td>
        <td>123.234.123.234</td>
    </tr>
    <tr>
        <td>ns3-for-laitos.my-home-domain-example.net</td>
        <td>A</td>
        <td>123.234.123.234</td>
    </tr>
    <tr>
        <td>ns4-for-laitos.my-home-domain-example.net</td>
        <td>A</td>
        <td>123.234.123.234</td>
    </tr>
</table>

The setup must have a minimum of two records because very often registrars ask for at least two name servers to be
assigned to a domain name, for redundancy reasons. Next, visit the registrar of `my-throw-away-domain-example.net`, set
its name servers to the following:
<table>
    <tr>
        <th>Registrar Field</th>
        <th>Value</th>
    </tr>
    <tr>
        <td>Name server 1 (mandatory)</td>
        <td>ns1-for-laitos.my-home-domain-example.net</td>
    </tr>
    <tr>
        <td>Name server 2 (mandatory)</td>
        <td>ns2-for-laitos.my-home-domain-example.net</td>
    </tr>
    <tr>
        <td>Name server 3 (perhaps optional)</td>
        <td>ns3-for-laitos.my-home-domain-example.net</td>
    </tr>
    <tr>
        <td>Name server 4 (perhaps optional)</td>
        <td>ns4-for-laitos.my-home-domain-example.net</td>
    </tr>
</table>

The DNS changes made above will take couple of hours to propagate through the Internet.

### Invoke app command
First, prepare the app command that is about to run:
1. Compose an app command, e.g. `mypassword.s echo 123`
2. Substitute all numbers and symbols with [DTMF input sequence table](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-Twilio-telephone-SMS-hook#usage).
   e.g. `mypassword1420s0echo0110120130`
3. Prepend the app command with an underscore `_` as prefix, e.g. `_mypassword1420s0echo0110120130`
4. If the command is longer than 63 characters, split it into segments of less than 63 characters each, and concatenate
   the segments with a dot, e.g. `_mypassword.1420s0.echo0110120130`. You pay split the command even if it is already
   sufficiently short.
5. Append the throw-away domain name at the end, e.g. `_mypassword.1420s0.echo0110120130.my-throw-away-domain-example.net`.

From any computer, mobile phone, or tablet, send the name query as a TXT record query. For example via via the `dig`
command on Linux:

    dig -t TXT _mypassword.1420s0.echo0110120130.my-throw-away-domain-example.net +timeout=30
    ; <<>> DiG 9.9.4-RedHat-9.9.4-61.amzn2.1.1 <<>> -t TXT _mypassword.1420s0.echo0110120130.my-throw-away-domain-example.net
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 33180
    ;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 0
    
    ;; QUESTION SECTION:
    ;_mypassword.1420s0.echo0110120130.my-throw-away-domain-example.net. IN TXT
    
    ;; ANSWER SECTION:
    _mypassword.1420s0.echo0110120130.my-throw-away-domain-example.net. 30 IN TXT "123"
    
    ;; Query time: 29 msec
    ;; SERVER: 10.12.0.2#53(10.12.0.2)
    ;; WHEN: Mon Feb 25 18:41:51 UTC 2019
    ;; MSG SIZE  rcvd: 167

The app command response (string `123` from our example) can be read in the `ANSWER SECTION`.

### Tips
- Respect and comply with the terms and policies imposed by your Internet service provider in regards to usage of DNS
  queries.
- DNS queries are not encrypted, your app command input (including password) and command output will be exposed to
  the public Internet. Only use DNS for app command invocation as a last resort when all other encrypted channels are
  unavailable.
- The entire DNS query, including app command, throw-away domain name, and dots in between, may not exceed 254 characters.
- The app command response from DNS query result has a maximum length of 254 characters.
- The DNS query response carrying app command response uses a TTL (time-to-live) of 30 seconds, which means, if an
  identical app command is issued within 30 seconds of the previous query, it will not reach laitos server, instead,
  the cached response from 30 seconds ago will arrive instantaneously.
- By default, each app command is given 29 seconds to complete unless the timeout duration is overriden by `PLT`
  command processor mechanism.
- Recursive DNS resolvers on the public Internet usually expects a query response from laitos in less than 10 seconds.
  However, in preactice, many app commands take more than 10 seconds to complete, in which case the public recursive
  DNS resolver will respond to DNS client with an error "(upstream) name server failure". Don't worry - internally,
  laitos patiently waits for the command completion (or time out), and makes the command response ready for immediate
  retrieval when the user invokes the identical app command within 30 seconds of the command completion.
