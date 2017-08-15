# Daemon: DNS server

## Introduction
The DNS server daemon provides an ad-free web experience.

It downloads the latest ad-domain list from well-known [yoyo.org](http://pgl.yoyo.org) and [mvps.org](http://winhelp2002.mvps.org)
on startup and then every 2 hours.

The daemon then forwards all name queries to a reputable public DNS of your choice; if a query is an advertisement domain,
it produces a black-hole answer (0.0.0.0) instead of forwarding the query. This effectively blocks most advertisements.

## Configuration
Construct the following JSON object and place it under key `DNSDaemon` in configuration file. All of them are mandatory:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The address network to listen to. It is usually "0.0.0.0", which means listen on all network interfaces.</td>
</tr>
<tr>
    <td>UDPPort</td>
    <td>integer</td>
    <td>UDP port number to listen on. It is usually 53 - the port number designated for DNS.</td>
</tr>
<tr>
    <td>UDPForwarder</td>
    <td>string</td>
    <td>
        If a UDP query is not for an advertisement domain, forward it to this public DNS service.
        <br/>
        Example: 8.8.8.8 by Google DNS, or 8.26.56.26 by Comodo.
    </td>
</tr>
<tr>
    <td>TCPPort</td>
    <td>integer</td>
    <td>TCP port number to listen to. It is usually 53 - the port number designated for DNS.</td>
</tr>
<tr>
    <td>TCPForwarder</td>
    <td>string</td>
    <td>
        If a TCP query is not for an advertisement domain, forward it to this public DNS service.
        <br/>
        Example: 8.8.8.8 by Google DNS, or 8.26.56.26 by Comodo.
    </td>
</tr>
<tr>
    <td>AllowQueryIPPrefix</td>
    <td>array of strings</td>
    <td>
        An array of IP address prefixes such as ["195.1", "123.4.5"] that are allowed to make DNS queries on the server.
        <br/>
        The public IP addresses of your computers and phones should be here.
    </td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>
        How many times in ten-second interval a client (identified by IP) is allowed to query the server.
        <br/>
        Each computer/phone usually uses about 15.
    </td>
</tr>
</table>

Here is an example setup made for two home devices (limit = 2 * 15) and forwards to Google public DNS. 

<pre>
{
    ...
    
    "DNSDaemon": {
        "Address": "0.0.0.0",

        "UDPPort": 53
        "UDPForwarder": "8.8.8.8",

        "TCPPort": 53
        "TCPForwarder": "8.8.4.4",

        "AllowQueryIPPrefixes": ["195", "35.196", "35.158.249.12"],
        "PerIPLimit": 30
    },
     
    ...
}
</pre>

## Run
Tell laitos to run DNS daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -frontend ...,dnsd,...

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

*Important tip*: DNS settings are usually memorised per network. Make sure to change DNS settings for all wireless and
wired networks.