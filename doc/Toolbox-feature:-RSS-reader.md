# Toolbox feature: RSS reader

## Introduction
Via any of enabled laitos daemons, you may read news feeds and briefings via RSS.

## Configuration
Under JSON object `Features`, construct a JSON object called `RSS` that has the following properties:
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
Use any capable laitos daemon to run command:

    .r skip count

Where `skip` is the number of latest feeds to discard, and `count` is the number of feeds to read after discarding.

# Tips
Upon running this command, the feeds are downloaded from all sources at once, sorted in chronological order from latest
to oldest, and then the `skip` and `count` parameters are taken into account.

If some of the sources failed to respond, the command response will still collect feeds from working sources. The
[system maintenance](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-system-maintenance) daemon helps to check validity
of the source URLs.