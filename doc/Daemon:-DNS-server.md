# Daemon: DNS server

## Introduction
The DNS server daemon is a recursive DNS resolver that provides a safer web experience by blocking most of advertisement
and malicious domains.

Every two hours, the blacklists are automatically updated from well-known sources:
- [yoyo.org](http://pgl.yoyo.org)
- [mvps.org](http://winhelp2002.mvps.org)
- [malwaredomainlist.com](http://www.malwaredomainlist.com)
- [someonewhocares.org](http://someonewhocares.org/hosts/hosts)

Beyond blacklist filter, the daemon uses redundant set of secure and trusted public DNS services provided by:
- [Comodo SecureDNS](https://www.comodo.com/secure-dns)
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
    <td>array of strings</td>
    <td>Public DNS resolvers (IP:Port) to use. They must be able to handle both UDP and TCP for queries.</td>
    <td>Comodo SecureDNS, Quad9, SafeDNS</td>
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
- Use a well-known public DNS service (one of the forwarders, for example) as backup DNS server in DNS settings, so that
  in the unlikely case of laitos DNS server going offline, your computers and phones will still be able to browse the
  Internet.

Regarding configuration:
- Not all DNS services support TCP for queries. The default forwarders (Comodo SecureDNS, Quad9, and SafeDNS) support
  both TCP and UDP very well.
- By specifying forwarders explicitly, the default forwarders will no longer be used.
