## Introduction
Ask about weather, math, physics, and all sorts of questions on WolframAlpha - the computational knowledge engine.

## Preparation
Create your very own WolframAlpha application:
1. Visit [WolframAlpha Developer Portal](https://developer.wolframalpha.com/portal/signin.html).
2. Sign up for an account and then sign in.
3. In [My Apps](https://developer.wolframalpha.com/portal/myapps/), click "Get an AppID".
4. Enter a name for your application. The name is not very important.
5. Note down "APPID" in the response.

## Configuration
Under JSON object `Features`, construct a JSON object called `WolframAlpha` that has the following mandatory properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>AppID</td>
    <td>string</td>
    <td>Your WolframAlpha application ID.</td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

        "WolframAlpha": {
            "AppID": "XXXXXX-1234567890"
        },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to invoke the app:

    .w this is a question for wolfram alpha to solve

A short moment later, WolframAlpha's answer will appear in the response.

## Tips
The application you created with WolframAlpha is for non-commercial use. As of 2017-11-07, you may use WolframAlpha for
up to 2000 times in a month for non-commercial application.

WolframAlpha is an incredibly powerful knowledge engine, it will happily answer questions from a large variety of fields.
