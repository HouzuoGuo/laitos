## Introduction
The web proxy daemon is a general-purpose HTTP proxy, capable of handling both HTTP and HTTPS destinations. It is especially suitable
for general web browsing on personal computing devices such as laptops and phones.

When the web proxy daemon and the [DNS daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-DNS-server) are enabled together,
the proxy daemon will provide protection against advertisements/tracking/malware by using the blacklist collected by the DNS daemon.

Optionally, to obtain insights of proxy destination performance and web browsing habits, the proxy can make these metrics available
in the web service of [prometheus metrics exporter](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-prometheus-metrics-exporter).

## Configuration
Construct the following JSON object and place it under key `HTTPProxyDaemon` in configuration file:

<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>AllowFromCidrs</td>
    <td>array of strings</td>
    <td>
        An array of CIDRs (e.g. "Your.Public.IP.xx/32") that are allowed to use this web proxy.
        <br/>
        Visitors that do not belong to any of these CIDRs will be refused proxy service.
    </td>
    <td>Empty (see also Tips for an alternative)</td>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The address network to listen on.</td>
    <td>"0.0.0.0" - listen on all network interfaces.</td>
</tr>
<tr>
    <td>Port</td>
    <td>integer</td>
    <td>The TCP port number to listen on.</td>
    <td>210</td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>
        Maximum number of proxy requests a client (identified by IP) may make in a second.
    </td>
    <td>100 - good enough for general web browsing from 4 devices simultaneously</td>
</tr>
</table>

Here is an example:

<pre>
{
    ...

    "HTTPProxyDaemon": {
        "AllowFromCidrs": ["35.158.249.12/32", "37.219.239.179/32"]
    },

    ...
}
</pre>

## Run
Tell laitos to run the HTTP web proxy daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,httpproxy,...

## Test (Optional)
Assuming that the proxy daemon is listening on the default port 210, perform this tests from a computer where you will be using the proxy
server: `curl -L -x http://LaitosServerHostNameOrIP:210 https://github.com`.

If the command runs successfully and gives plenty of HTML output, then the test has successfully passed, and you are ready to use the web proxy
on personal computing devices.

## Usage
On your personal computing devices (such as phones and laptops), visit OS network settings and then set:

- Use a proxy server: Yes
- Proxy server address: `http://LaitosServerHostNameOrIP`
- Proxy server port: `210` (or the custom port you set in configuration)

If presented to you, set these settings too:
- Use the proxy server except for these addresses: `*.local` and your home router's local domain name.
- Don't use the proxy server for local (intranet) addresses: Yes

For other cases such as Linux command line, set environment variable `http_proxy` and `https_proxy` to `http://LaitosServerHostNameOrIP:Port`.
Majority of Linux programs obey the two environment variables.

Optionally, if you wish to obtain some insights about data transferred over this proxy and users' browsing habits, such as getting the top N proxy
destinations by data transfer and number of connections, then turn on on prometheus integration (`sudo ./laitos -prominteg -config ... -daemons ...httpproxy,httpd,...`)
and enable the web server to serve the [prometheus metrics exporter](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-prometheus-metrics-exporter).
See the exporter's tips for examples of useful queries as such.

## Tips
- Instead of manually figure out your home public IP and placing it into `AllowFromCidrs`, run [phone-home telemetry daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry)
  on a computer inside that network (e.g. on a laptop or desktop) and configure the daemon to send reports to the laitos server.
  The web proxy daemon will automatically allow all telemetry subjects to freely use the proxy.
