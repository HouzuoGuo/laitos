## Introduction

Post Tweets and read tweets from home time-line.

## Preparation

Create your very own Twitter application:

1. Visit [Twitter Developers](https://dev.twitter.com/).
2. Navigate to [My apps](https://apps.twitter.com/).
3. Proceed to create a new application, fill in the name, description, and website. Leave callback URL empty.

And then, obtain an API access token:

1. Visit your newly created app, navigate to "Key and Access Tokens" tab.
2. Note down your "Consumer Key (API Key)" and "Consumer Secret (API Secret)".
3. Click "Create my access token".
4. Note down your "Access Token" and "Access Token Secret".

The keys and secrets do not expire. They will be used in laitos configuration.

## Configuration

Under JSON object `Features`, construct a JSON object called `Twitter` that has the following mandatory properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>AccessToken</td>
    <td>string</td>
    <td>API access token.</td>
</tr>
<tr>
    <td>AccessTokenSecret</td>
    <td>string</td>
    <td>API access token secret.</td>
</tr>
<tr>
    <td>ConsumerKey</td>
    <td>string</td>
    <td>Consumer (application) API key.</td>
</tr>
<tr>
    <td>ConsumerSecret</td>
    <td>string</td>
    <td>Consumer (application) API secret.</td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

        "Twitter": {
            "AccessToken": "12345678-erngbfuninxkjnxvvvvveruihuiuiersuiiidf",
            "AccessTokenSecret": "iowa4jiojiobniofgnncvbmknbnyrtubyt",
            "ConsumerKey": "fxoieprkpokpowwwwmcgbmkk",
            "ConsumerSecret": "xvm,mbrypziweijzwemimfdrtgjkbmfgmkkm"
        },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to invoke the app:
- Post a tweet: `.tp tweet content`, a short moment later the command response will say a number that is length of
  tweet.
- Read latest tweets from home time-line: `.tg skip count`, where `skip` is the number of latest tweets to discard, and
  `count` is the number of tweets to read after discarding.

Be aware: Twitter will only allow retrieving up to 200 latest tweets, that means `skip + count` may not exceed 200.
