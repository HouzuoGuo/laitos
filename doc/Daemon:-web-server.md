# Daemon: web server

## Introduction
The web server hosts a static personal website that consists of:
- A home page in an HTML file.
- Media files and other assets in directories.

Additionally, specialised web services such as [Twilio telephone/SMS hook](https://github.com/HouzuoGuo/laitos/wiki/Web-service:-Twilio-telephone-SMS-hook)
are also hosted by the web server.

## Configuration
Construct the following JSON object and place it under JSON key `HTTPDaemon` in configuration file:
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
    <td>Port</td>
    <td>integer</td>
    <td>Port number to listen on. It is usually 443 for TLS-enabled server, or 80 for non-TLS server.</td>
    <td>443 (if TLS is enabled) or 80 (if TLS is not enabled)</td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>
        Maximum number of visits a visitor (identified by IP) may make in a second.
        <br/>
        The number acts as a multiplier in initialising rate limit of file, directory, and web service access.
    </td>
    <td> 12 - resonable for a personal website</td>
</tr>
<tr>
    <td>ServeDirectories</td>
    <td>{"/the/url/location": "/path/to/directory"...}</td>
    <td>Serve the directories at the specified URL location. The prefix slash in URL location string is mandatory.</td>
    <td>(Not used by default)</td>
</tr>
<tr>
    <td>TLSCertPath</td>
    <td>string</td>
    <td>
        Absolute or relative path to PEM-encoded TLS certificate file.
        <br/>
        The file may contain a certificate chain with server certificate on top and CA authority toward bottom.
    </td>
    <td>(Not enabled by default)</td>
</tr>
<tr>
    <td>TLSKeyPath</td>
    <td>string</td>
    <td>Absolute or relative path to PEM-encoded TLS certificate key.</td>
    <td>(Not enabled by default)</td>
</tr>
</table>

### Host home page (index page)
To host a home page, place the following things under JSON key `HTTPHandlers` in configuration file:

- String array `IndexEndpoints` - URL locations that will serve home page; in most cases it should be:

      ["/", "/index.html"]

  The prefix slash is mandatory.
- Object `IndexEndpointConfig` that comes with the following mandatory attributes:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>HTMLFilePath</td>
    <td>string</td>
    <td>Path to HTML home page file.</td>
</tr>
</table>


### Example
Here is an example setup that hosts a home page and media files:
<pre>
{
    ...

    "HTTPDaemon": {
        "TLSCertPath": "howard-dot-net.crt",
        "TLSKeyPath": "howard-dot-net.key",
        "ServeDirectories": {
            "/media/videos": "/home/howard/CoolVideos",
            "/site/img": "/home/howard/WebsiteImages",
            "/site/css": "/home/howard/WebsiteCSS",
            "/site/js": "/home/howard/WebsiteJavascript",
        }
    },
    "HTTPHandlers": {
        ...

        "IndexEndpointConfig": {
            "HTMLFilePath": "index.html"
        },
        "IndexEndpoints": ["/", "/index.html"],

        ...
    },

    ...
}
</pre>

## Run
Tell laitos to run web server in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemonssimple ...,httpd,...

If TLS is enabled on the web server and the site should also be accessible via plain HTTP, then tell laitos to start one
more server:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,httpd,insecurehttpd,...

The plain HTTP server listens on hard-coded port 80, it shares configuration with the TLS-enabled web daemon, which
means it serves all HTML files, file directories, and special handles.

## Deployment
In order for an Internet user to browse your website hosted via laitos:
1. Your domain names must be covered by a DNS hosting service. If the concept sounds unfamiliar, check out this article
    from Amazon Web Service: [What is DNS](https://aws.amazon.com/route53/what-is-dns/).
2. DNS hosting providers usually charge ~ 1 USD per domain per month. If you are looking for a provider, check out:
   - [Amazon Web Service "Route53"](https://aws.amazon.com/route53/)
   - [Google Cloud Platform "Cloud DNS"](https://cloud.google.com/dns/)
3. Check at your Domain Registrar that the domain name servers are pointing to DNS hosting providers.
4. If you are making changes to domain name servers, it may take up to 24 hours to propagate through the Internet.

Now, create or modify a DNS "A" entry for your domain name. The entry must look like:

- DNS name: `my-domain-name.net`
- Record type: `A`
- Time to live (TTL): leave at default or `5 minutes`
- Value (preference and mail server): the public IP address of laitos server

Here is an example involving two domain names and one sub-domain, assuming that laitos server is on `58.169.236.112`:

<table>
<tr>
    <th>DNS name</th>
    <th>Record type</th>
    <th>Time to live (TTL)</th>
    <th>Value</th>
    <th>Remark</th>
</tr>
<tr>
    <td>howard-homepage.net</td>
    <td>A</td>
    <td>5 minutes</td>
    <td>58.169.236.112</td>
    <td>First example</td>
</tr>
<tr>
    <td>howard-blog.org</td>
    <td>A</td>
    <td>5 minutes</td>
    <td>58.169.236.112</td>
    <td>Second example</td>
</tr>
<tr>
    <td>cool.howard-blog.org</td>
    <td>A</td>
    <td>5 minutes</td>
    <td>58.169.236.112</td>
    <td>A sub-domain of second example</td>
</tr>
</table>

Wait up to an hour for new DNS records to propagate through the Internet.

## Test
Use a web browser to visit laitos web server on your domain name, pay special attention to these items:
- If TLS is enabled, connect to web server via HTTPS, and browser should recognise the TLS certificate.
- Visit home page locations.
- Visit file directories - it is normal for browser to see a directory file listing page.
- If plain HTTP daemon is enabled, check home page and file directories on HTTP port 80 as well.

## Tips
1. The home page HTML is slightly processed in this way:
    - `#LAITOS_CLIENTADDR` is substituted to visitor's IP address.
    - `#LAITOS_3339TIME` is substituted to current system date and time.

    For example, the following HTML snippet:

        <p>Welcome, visitor! Your IP is #LAITOS_CLIENTADDR and the time is now #LAITOS_3339TIME.</p>

    Will be rendered as (IP and time are examples):

        <p>Welcome, visitor! Your IP is 41.156.72.9 and the time is now 2017-08-22T15:04:05Z07:00</p>
2. When you access specialised web services via the plain HTTP daemon, your will be warned about this usage of
   unencrypted HTTP connection. The warning comes in an authentication dialog that accepts any username password input.
   As an exception, visiting home page and file directories do not trigger the warning.