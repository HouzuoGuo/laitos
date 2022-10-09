## Introduction

In addition to providing your home network a safer and cleaner web experience,
the [DNS server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server)
is also capable of tunneling TCP connections (TCP-over-DNS) and tunneling DNS
queries (DNS-over-TCP-over-DNS).

This enables a mode of low-bandwidth (~1KB/s) Internet browsing experience in a
restricted network where normal TCP/IP communication is unavailable - which is
rather common with in-flight WiFi and hotspots with captive portals.

## Demo

In the demo, laitos starts a localhost HTTP(S) proxy, the proxy clients are
connected to laitos DNS server via the TCP-over-DNS mechanism. The web browser
running in the terminal then nagivates to wikipedia home page.

```text
Web browser --> localhost HTTP(S) proxy --> TCP-over-DNS transport |
 (links2)           (laitos CLI)                (laitos CLI)       |
                                                                   v
                                                              DNS queries
                                                                   |
                                                                   |
                                                                   v
                       laitos DNS daemon <-- Internet <-- Recursive DNS resolver
```

[![asciicast](https://asciinema.org/a/526718.svg)](https://asciinema.org/a/526718)

## Configuration

First, add the [DNS server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server)
daemon configuration (`DNSDaemon`) to the JSON config file.

Make sure to specify laitos server(s)' own domain or sub-domain names in
`MyDomainNames` - laitos will handle TCP-over-DNS tunnels for these domain names
on its own, without forwarding them.

Then construct the following object under key `DNSDaemon`, `TCPProxy`:

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>RequestOTPSecret</td>
    <td>string</td>
    <td>
        A password-like string to authorise new connection requests.
        <br/>
        Later in Usage, we will use the same password to launch TCP-over-DNS
        clients - the localhost HTTP(S) proxy and DNS proxy resolver.
    </td>
    <td>Empty (TCP-over-DNS unavailable)</td>
</tr>
</table>

Here is a complete example:

<pre>
{
    ...

    "DNSDaemon": {
        "MyDomainNames": ["my-laitos-server.com", "my-laitos-server.net"],
        "TCPProxy": {
            "RequestOTPSecret": "tcpoverdns-password"
        }
    },

    ...
}
</pre>

## Route TCP-over-DNS queries from your domain name to laitos DNS server

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

The TCP-over-DNS proxy is built into the DNS server daemon. Run the DNS daemon
by specifying it in the laitos command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,dnsd,...

## Usage

The laitos executable has both the server (`dnsd`) and the client built-in.

### Browse the web via a localhost HTTP(S) proxy

The localhost (default address `127.0.0.12:8080`) proxy is compatible with all
web browsers. It proxies HTTP and HTTPS request via TCP-over-DNS toward your
laitos DNS server.

Start the proxy by launching the laitos executable with these CLI flags:

<table>
<tr>
    <th>Flag</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>-proxydnsname</td>
    <td>string</td>
    <td>The (sub)domain name of your laitos DNS server.</td>
    <td>Mandatory without default</td>
</tr>
<tr>
    <td>-proxyotpsecret</td>
    <td>string</td>
    <td>
        The authorisation string for new connection requests.
        <br/>
        It must match the "RequestOTPSecret" used on the laitos DNS server.
    </td>
    <td>Mandatory without default</td>
</tr>
<tr>
    <td>-proxyport</td>
    <td>number</td>
    <td>The port number of the HTTP(S) proxy.</td>
    <td>8080</td>
</tr>
<tr>
    <td>-proxyresolver</td>
    <td>"ip:port" string</td>
    <td>The recursive resolver address.</td>
    <td>Name servers from /etc/resolv.conf</td>
</tr>
<tr>
    <td>-proxyseglen</td>
    <td>integer</td>
    <td>The maximum segment length.</td>
    <td>Automatic</td>
</tr>
</table>

Example:

    ./laitos -proxydnsname=sub.laitos-example.com -proxyotpsecret=tcpoverdns-password

Configure your web browser to use the address `127.0.0.12:8080` (the port number
comes from `-proxyport`) for both the HTTP and HTTPS proxy.

Each proxy connection has a sustained throughput of ~1.2KB/second under normal
conditions, therefore using a terminal-based web browser (such as `links2`) is
strongly recommended.

On the other hand, desktop browsers often download large javascript and image
files unsuitable for the limited throughput.

### Start a companion localhost DNS proxy resolver

The localhost DNS proxy (`127.0.0.12:53`) proxies DNS requests via TCP-over-DNS
toward your laitos DNS server.

The DNS proxy is capable of handling both the regular UDP DNS requests as well
as DNS-over-TCP requests - that's DNS-over-TCP-over-TCP-over-DNS for you!

Start the DNS proxy by starting the localhost HTTP(S) proxy with additional
CLI flags:

- `-proxyrelaydns` - turn on DNS proxy.
- `-proxyresolver` - specify the recursive resolver (`ip:port`). This is usually
  the default DNS resolver address in your local area network.

Example:

    sudo ./laitos -proxydnsname=sub.laitos-example.com -proxyotpsecret=tcpoverdns-password -proxyrelaydns -proxyresolver 192.168.0.1:53

You may need to use `sudo` as the DNS proxy listens on the privileged port 53.

Next, change `/etc/resolv.conf`, remove all of the name servers and add
`127.0.0.12:53`. This forces all DNS requests to go through the proxy.

## Tips

Please respect and comply with the terms and conditions of your Internet
service and captive portal service providers.

If laitos CLI complains that outgoing DNS requests fail, then there is a small
likelihood that the automatically calculated segment length (printed at startup)
is too large. Try shrinking the maximum segment length using the `-proxyseglen`
CLI parameter.

## Acknowledgements

The idea of tunneling data over DNS took some inspiration from the [yarrick/iodine](https://github.com/yarrick/iodine)
project. However, while iodine is an IP-over-DNS (layer-3) proxy, laitos DNS
server implements a TCP-over-DNS (layer-4) proxy.
