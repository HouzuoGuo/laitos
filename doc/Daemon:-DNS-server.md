# Daemon: DNS server

## Introduction
The DNS server daemon is a recursive DNS resolver that provides a safer web experience by blocking most of advertisement
and malicious domains.

Upon start up and twice a day, the blacklists are automatically acquired from well-known sources:
- [malwaredomainlist.com](http://www.malwaredomainlist.com)
- [someonewhocares.org](http://someonewhocares.org/hosts/hosts)
- [mvps.org](http://winhelp2002.mvps.org)
- [yoyo.org](http://pgl.yoyo.org)

Beyond blacklist filter, the daemon uses redundant set of secure and trusted public DNS services provided by:
- [OpenDNS](https://www.opendns.com)
- [Quad9](https://www.quad9.net)
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
    <td>array of "IP:port"</td>
    <td>Public DNS resolvers (IP:Port) to use. They must be able to handle both UDP and TCP for queries.</td>
    <td>OpenDNS, Quad9, SafeDNS</td>
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
- Not all DNS services support TCP for queries. The default forwarders (Quad9, SafeDNS, OpenDNS) support both TCP and
  UDP very well.
- By specifying forwarders explicitly, the default forwarders will no longer be used.

## Use DNS server to run toolbox commands
Beside offering an ad-free and safe web experience, the DNS server also runs toolbox commands via `TXT` queries, this
enables Internet usage in an environment where DNS usage is unrestricted but Internet access is not available.

### Configuration
To enable toolbox command execution, follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor)
to construct configuration for JSON key `DNSFilters`, for example:

<pre>
{
    ...

    "DNSDaemon": {
        "AllowQueryIPPrefixes": ["195", "35.196", "35.158.249.12"]
    },
    "DNSFilters": {
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
1. The first domain name will offer toolbox command execution capability, but will not be able to host anything else.
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

### Execute toolbox command
After DNS changes have been propagated, compose a toolbox command as usual, except that numbers and symbols input must
be substituted by [DTMF input sequence table](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-Twilio-telephone-SMS-hook#usage).
Prepend the substituted command input with an underscore `_` as prefix, and append the first domain name as suffix.

For example, to input a toolbox command that prints "123" `verysecretpassword.s echo 123`:
1. Find the DTMF input corresponding to full-stop (DTMF 142), space (DTMF 0), and digits (DTMF 11, 12, 13). Keep in mind
   that according to DTMF input rules, extra DTMF input `0` needs to be used to mark each instance of DTMF sub-phrase.
2. Substitute all of the symbols, spaces, and digits with DTMF input sequence: `veryverysecretpassword1420s0echo0110120130`.
3. Prepend an underscore `_` as prefix: `_veryverysecretpassword1420s0echo0110120130`
4. Append the first domain name as suffix:  `_veryverysecretpassword1420s0echo0110120130.my-throw-away-domain-example.net`

From any computer, mobile phone, or tablet, send the name query as a TXT record query. For example via via the `dig`
command on Linux:

    dig -t TXT _veryverysecretpassword1420s0echo0110120130.my-throw-away-domain-example.net +timeout=30
    ; <<>> DiG 9.9.4-RedHat-9.9.4-61.amzn2.1.1 <<>> -t TXT _veryverysecretpassword1420s0echo0110120130.my-throw-away-domain-example.net
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 33180
    ;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 0
    
    ;; QUESTION SECTION:
    ;_veryverysecretpassword1420s0echo0110120130.my-throw-away-domain-example.net. IN TXT
    
    ;; ANSWER SECTION:
    _veryverysecretpassword1420s0echo0110120130.my-throw-away-domain-example.net. 30 IN TXT "123"
    
    ;; Query time: 29 msec
    ;; SERVER: 10.12.0.2#53(10.12.0.2)
    ;; WHEN: Mon Feb 25 18:41:51 UTC 2019
    ;; MSG SIZE  rcvd: 167

The toolbox command execution result can be read from the `ANSWER SECTION`. 

Keep in mind that:
- DNS queries are not encrypted, your toolbox command input (including password) and command output will be exposed to
  the public Internet. Only use this toolbox command access as a last resort when all other encrypted channels are
  unreachable.
- There is a hard limit of 255 characters in the laitos DNS response, enable all text linting options in `LintText`
  configuration to make command output more readable.
- Respect and comply with the terms and policies imposed by your Internet service provider in regards to usage of DNS
  queries.
