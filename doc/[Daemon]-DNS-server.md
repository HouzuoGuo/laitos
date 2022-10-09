## Introduction

The DNS server daemon is a recursive DNS resolver, it blocks most advertising
and malicious domains to provide your home network a safer and cleaner web
experience.

The domain blacklists are automatically updated from these sources:

- [malwaredomainlist.com](http://www.malwaredomainlist.com)
- [someonewhocares.org](http://someonewhocares.org/hosts/hosts)
- [mvps.org](http://winhelp2002.mvps.org)
- [yoyo.org](http://pgl.yoyo.org)
- [oisd.nl light hosts list](https://oisd.nl/)
- [The Block List Project (ransomware/scam/tracking)](https://github.com/blocklistproject/Lists)

At the same time, the DNS server daemon is also capable of:

- Invoking app commands via exchanging TXT records - [DNS server (invoke app commands)](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server-(invoke-app-commands)).
- Tunneling TCP traffic over DNS queries - [DNS server (TCP over DNS)](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server-(TCP-over-DNS)).

## Configuration

Construct the following JSON object and place it under key `DNSDaemon` in the
JSON config file:

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>AllowQueryFromCidrs</td>
    <td>array of CIDR blocks</td>
    <td>
        An array of client network address blocks in CIDR notation. The laitos
        DNS server will only process recursive queries from these CIDR blocks.
        <br/>
        Your ISP may assign a random public IP IP from a larger block to your
        home network. Find out your public IP from Google (<a href="https://www.google.com/search?q=what+is+my+ip">What is my IP</a>).
        Be generous/flexible with the block size - /16 is a good starting point.
        <br/>
        This restriction does not apply to non-recursive queries, such as <a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server-(invoke-app-commands)">DNS server (invoke app commands)</a>
        or <a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server-(TCP-over-DNS)">DNS server (TCP over DNS)</a>.
    </td>
    <td>Empty</td>
</tr>
<tr>
    <td>MyDomainNames</td>
    <td>array of strings</td>
    <td>
        The laitos DNS server's own domain names.
        This is used by <a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server-(invoke-app-commands)">DNS server (invoke app commands)</a>
        and <a href="https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server-(TCP-over-DNS)">DNS server (TCP over DNS)</a>
        to determine whether to handle a query on its own or send it to a
        forwarder.
    </td>
    <td>Empty</td>
</tr>
<tr>
    <td>Forwarders</td>
    <td>array of "IP:port" strings</td>
    <td>
        The upstream resolvers (public DNS resolvers). They must be able to
        handle both UDP and TCP for queries.
        <br/>
        Specify more than one resolvers for improved performance and redundancy.
    </td>
    <td>
        <a href="https://www.quad9.net">Quad9</a>,
        <a href="https://blog.cloudflare.com/introducing-1-1-1-1-for-families/">CloudFlare</a>,
        <a href="https://www.opendns.com">OpenDNS</a>, and <a href="https://adguard.com/en/adguard-dns/overview.html">AdGuard DNS</a>.</td>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The network address to listen on.</td>
    <td>"0.0.0.0" - listen on all network interfaces.</td>
</tr>
<tr>
    <td>Forwarders</td>
    <td>array of "IP:port" strings</td>
    <td>
        The upstream resolvers (public DNS resolvers). They must be able to
        handle both UDP and TCP for queries.
        <br/>
        Specify more than one resolvers for improved performance and redundancy.
    </td>
    <td>Quad9, CloudFlare, OpenDNS, and AdGuard DNS.</td>
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
        Process a maximum of this number of queries per second from each client.
        Each client is identified by its IP address.
        <br/>
    </td>
    <td>50 - good for 3 personal devices</td>
</tr>
</table>

Here is a minimal JSON config file example:

<pre>
{
    ...

    "DNSDaemon": {
        "AllowQueryFromCidrs": ["35.196.0.0/16", "37.228.0.0/16"]
    },

    ...
}
</pre>

### Configuration tips

Instead of manually figure out your home public IP and placing it into `AllowQueryFromCidrs`,
run [phone-home telemetry daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry)
on a computer inside that network (e.g. on a laptop or desktop) and configure
the daemon to send reports to this laitos server. All telemetry subjects
are automatically allowed to use the DNS server without restrictions.

## Run

Run the DNS daemon by specifying it in the laitos command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,dnsd,...

## Test (Optional)

Assuming that daemon listens on port 53, try out these tests from your home
network:

1. Observe successful "Name-Address" answers from the following system commands:

        nslookup microsoft.com <LAITOS SERVER IP>
        nslookup -vc microsoft.com <LAITOS SERVER IP>

2. Observe a black-hole answer `0.0.0.0` from the following system command:

        nslookup analytics.google.com <LAITOS SERVER IP>
        nslookup -vc analytics.google.com <LAITOS SERVER IP>

You may also run the test queries on the laitos server locally by using
`127.0.0.1` for the server IP address.

### Troubleshooting

If the tests are not successful, check laitos log. If the log says
`client IP is not allowed to query` then check the configuration value of
`AllowQueryFromCidrs`, make sure the CIDR blocks include your home network's
public IP.

If the tests are not successful, and laitos log says `client IP is not allowed to query`,
then double check that your public IP is included in one of the CIDR blocks of
`AllowQueryFromCidrs` from JSON config.

## Usage

Change the DNS settings of your mobile and desktop devices at home, set the DNS
server address to the public address of laitos DNS server.

Check out these tutorials:

- Windows/Mac tutorial by [Google](https://developers.google.com/speed/public-dns/docs/using#change_your_dns_servers_settings)
  * Alternative Windows tutorial by [windowscentral.com](https://www.windowscentral.com/how-change-your-pcs-dns-settings-windows-10)
- Android tutorial by [OpenDNS](https://support.opendns.com/hc/en-us/articles/228009007-Android-Configuration-instructions-for-OpenDNS)
- iOS/iPadOS tutorial by [appleinsider.com](https://appleinsider.com/articles/18/04/22/how-to-change-the-dns-server-used-by-your-iphone-and-ipad)

