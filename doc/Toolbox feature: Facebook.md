# Toolbox feature: Facebook

## Introduction
Via any of enabled laitos daemons, you may post updates to Facebook timeline.

## Preparation
Create your very own Facebook application:
1. Visit [Facebook for developers](https://developers.facebook.com/).
2. Use "My apps" and then "Add a new app".
3. Visit app's Dashboard tab and note down App ID and App Secret.
4. Visit app's "App Review" and turn on "Make app public". This is a very important step! If not done, your posts will
   not be visible to friends.

And then, obtain an API access token:
1. Visit [GraphAPI explorer](https://developers.facebook.com/tools/explorer/145634995501895/).
2. Under the application selector, choose your new application.
3. Click "Get Token", and then "Get User Access Token".
4. Under "User Data Permissions", turn on "publish_actions".
5. Click "Get Access Token" and proceed to grant the application access.
6. Note down the newly obtained "Access Token".

Stay in [GraphAPI explorer](https://developers.facebook.com/tools/explorer/145634995501895/) and refresh the newly
obtained API Access Token:
1. Put `oauth/access_token?grant_type=fb_exchange_token&client_id=<YOUR APP ID>&client_secret=<YOUR APP SECRET>&fb_exchange_token=<EXISTING ACCCESS TOKEN>`
   into the URL input.
2. Click Submit.
3. Note down the new "access_token" from Facebook response. This is your latest and best API access token.

API access tokens have short life (~59 days). You must refresh the very first token, and then refresh subsequent tokens
before they expire.

Remember to update laitos configuration with your latest Facebook token.

## Configuration
Under JSON object `Features`, construct a JSON object called `Facebook` that has the following mandatory properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>UserAccessToken</td>
    <td>string</td>
    <td>Your latest API access token.</td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

        "Facebook": {
            "UserAccessToken": "vsnm435oiungdnuiuvmesims398c389huidrnixdfnseee089nqw"
        },
        
        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to run the following toolbox command:

    .f message

Where `message` is to be posted to Facebook time-line. A short moment later, the command response will say a number that
is length of the message.

## Tips
Facebook annoyingly hands out these short-lived API access tokens that we must refresh each month, there is no way
around it.