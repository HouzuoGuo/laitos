## Introduction

Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server),
users may initiate a chat (e.g. Skype) with laitos to invoke app commands

The web hook is compatible with the Microsoft Bot Framework (aka Azure Bot
services).

## Preparation

1. Sign up for a Microsoft Azure account, navigate to your Resource Group, and
   create a new [Azure Bot](https://docs.microsoft.com/en-us/azure/bot-service/bot-service-quickstart-registration?view=azure-bot-service-4.0&tabs=userassigned).
   Choose "Multi Tenant" for the Microsoft App ID.You may wish to change the
   Pricing Tier choice from "Standard" to "Free" for a start.
2. Navigate to the "Bot profile" tab of the newly created bot and complete the
   profile details.
3. Navigate to the "Configuration" tab of the newly created bot, craft a new URL
   on your laitos server that will serve as the bot's web hook, and write it down
   into "Message endpoint" (e.g. `https://laitos.example.com/bot-web-hook`).
   Note down the URL and "Microsoft App ID" for use later in laitos
   configuration.
4. Navigate to the "Channels" tab of the newly created bot and connect the bot
   to end-user channels as you wish, such as Skype.

Next, navigate to the Azure Key Vault that was automatically created alongside
the new bot and retrieve the application ID and :

1. Visit the key vault's "Access policies" and add a new policy to grant
   yourself all permissions in the key vault.
2. Navigate to "Secrets" and inspect the current version of the only secret
   entity.
3. Click "Show Secret Value" to reveal the application secret. Note it down for
   use later in laitos configuration.

## Configuration

1. Place the following JSON data under JSON key `HTTPHandlers`:
    - String `MicrosoftBotEndpoint1` - URL location that will serve Microsoft bot; keep it a secret to yourself, and
      make it difficult to guess.
    - Object `MicrosoftBotEndpointConfig1` that comes with the following mandatory properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>ClientAppID</td>
    <td>string</td>
    <td>"Microsoft App ID" displayed in the Configuration tab of the Azure Bot.</td>
</tr>
<tr>
    <td>ClientAppSecret</td>
    <td>strings</td>
    <td>The secret value of the secret entity in the key vault automatically created alongside the new bot..</td>
</tr>
</table>

2. Follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct configuration for
   JSON key `HTTPFilters`.

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "MicrosoftBotEndpoint1": "/very-secret-microsoft-bot-hook",
        "MicrosoftBotEndpointConfig1": {
            "ClientAppID": "abcde-fghijkl-mnopqrs-xyz012",
            "ClientAppSecret": "b54xni73chmixdd9as3288"
        },

        ...
    },

    ...
}
</pre>

## Run

The service is hosted by web server, therefore remember to [run the web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

As required by Azure bot service, the web server must have a valid TLS
certificate and present its complete certificate chain to HTTPS clients.

## Usage

Visit [Azure Bot services](https://portal.azure.com/#blade/Microsoft_Azure_ProjectOxford/AppliedAIHub/BotServices),
click on the bot name, and visit the `Channels` tab. Click `Add to Skype` to
add the bot as a Skype contact.

Initiate a chat with the Skype contact, enter password followed by app command,
and the command result will arrive in a chat reply.

## Tips

- Make the endpoint URLs to guess, this helps to prevent misuse of the service.
- A laitos server may serve up to three bots. To configure more than one bot,
  fill in the configuration for `MicrosoftBotEndpoint2`, `MicrosoftBotEndpoint3`,
  as well as `MicrosoftBotEndpointConfig2` and `MicrosoftBotEndpointConfig3`.
