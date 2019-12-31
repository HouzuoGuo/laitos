## Introduction
Dial a friend's phone number, speak a text message, and send then SMS texts.

## Preparation
1. Sign up for an account at [twilio.com](https://www.twilio.com) - an API platform that connects computer programs with
   public switched telephone and mobile network. Sign up is free.
2. Visit Twilio developer's console, then [purchase a phone number](https://www.twilio.com/console/phone-numbers/search).
   Make sure the number can make calls and SMS - not all numbers come with these capabilities! A number costs between
   2-10 USD/month to own, and each call/SMS costs extra.
3. Visit [account settings](https://www.twilio.com/console/account/settings) and note down "Account SID" and
   "Auth Token" of LIVE Credentials. Do not use TEST credentials, and there is no need to request a secondary token.
4. Visit [phone numbers](https://www.twilio.com/console/phone-numbers/incoming) and note down the phone number including
   country code. Later this number will be used to dial calls and send SMS.

If you have or plan to use [web service hook for Twilio telephone and SMS](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-make-calls-and-send-SMS),
feel free to share the Twilio account and phone number with the web service as well.

## Configuration
Under JSON object `Features`, construct a JSON object called `Twilio` that has the following mandatory properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>AccountSID</td>
    <td>string</td>
    <td>Account SID of Twilio LIVE credentials.</td>
</tr>
<tr>
    <td>AuthToken</td>
    <td>string</td>
    <td>Auth Token of Twilio LIVE credentials.</td>
</tr>
<tr>
    <td>PhoneNumber</td>
    <td>string</td>
    <td>
        Your purchased Twilio phone number including +country code.<br/>
        Do not place additional space and symbol among the numbers.
    </td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

         "Twilio": {
              "AccountSID": "AC00000000111112222222222333333",
              "AuthToken": "689781347878abcdefg895897892342",
              "PhoneNumber": "+35815123456789"
            },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to invoke the app:

- Leave a voice message: `.pc +123456789 this is the voice message content`. Make sure the destination number comes with
  country code and there is no extra space or symbol among the numbers. The message will be spoken and repeated twice.
- Send an SMS: `.pt +123456789 this is the text message content`. Make sure the destination number comes with country
  code and there is no extra space or symbol among the numbers.
