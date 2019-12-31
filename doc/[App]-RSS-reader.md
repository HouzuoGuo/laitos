## Introduction
Read latest news and briefings via RSS.

## Configuration
This app is always available for use and does not require configuration.

However, if wish to override the default news sources, under JSON object `Features`, construct a JSON
object called `RSS` that has the following properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>Sources</td>
    <td>array of strings</td>
    <td>URLs to various RSS sources.</td>
    <td>Top stories/home page from A(ustralia)BC, BBC, Reuters, The Guardian, CNBC, and Jerusalem Post.</td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

         "RSS": {
              "Sources": [
                  "http://feeds.reuters.com/reuters/topNews",
                  "https://www.theguardian.com/uk/rss"
              ]
            },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to invoke the app:

    .r skip count

Where `skip` is the number of latest feeds to discard, and `count` is the number of feeds to read after discarding.

# Tips
Upon running this command, the feeds are downloaded from all sources at once, sorted in chronological order from latest
to oldest, and then the `skip` and `count` parameters are taken into account.

If some of the sources failed to respond, the command response will still collect feeds from the remaining working sources.
The program health report produced by [system maintenance](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance)
daemon helps to discover invalid source URLs.
