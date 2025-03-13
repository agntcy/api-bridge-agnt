As a usage example, we will use the API Bridge Agnt to send email via SENGRID API.

## Prequisites

- Get an API Key for free from sendgrid [sengrid by twilio](https://sendgrid.com/en-us)
- Retreive the open api spec here [tsg_mail_v3.json](https://github.com/twilio/sendgrid-oai/blob/main/spec/json/tsg_mail_v3.json)
- Make sure redis is running (otherwise, use `make start_redis`)
- Make sure you properly export `OPENAI_*` parameters
- Start the plugin as described on "Getting Started" section

## Update the API with tyk middleware settings

You need to add on the API the parameters allowing the plugin to use it.

```json
{
  [...]
  "x-tyk-api-gateway": {
    "info": {
      "id": "tyk-sendgrid-id",
      "name": "Sendgrid Mail API",
      "state": {
        "active": true
      }
    },
    "upstream": {
      "url": "https://api.sendgrid.com"
    },
    "server": {
      "listenPath": {
        "value": "/sendgrid/",
        "strip": true
      }
    },
    "middleware": {
      "global": {
        "pluginConfig": {
          "data": {
            "enabled": true,
            "value": {
            }
          },
          "driver": "goplugin"
        },
        "postPlugins": [
          {
            "enabled": true,
            "functionName": "SelectAndRewrite",
            "path": "middleware/agent-bridge-plugin.so"
          },
          {
            "enabled": true,
            "functionName": "RewriteQueryToOas",
            "path": "middleware/agent-bridge-plugin.so"
          }
        ],
        "responsePlugins": [
          {
            "enabled": true,
            "functionName": "RewriteResponseToNl",
            "path": "middleware/agent-bridge-plugin.so"
          }
        ]
      }
    }
  },
  [...]
```

You have an example of a such configuration here ```./configs/api.sendgrid.com.oas.json```

## Configure an endpoint to allow plugin to retrieve it

On the same oas file, add a "x-nl-input-examples" element to an endpoint with
sentence that describe how you can use the endpoint with natural language.
For example:

```json
{
  [...]
  "paths": {
    [...]
    "/v3/mail/send": {
      [...]
      "post": {
        [...]
        "x-nl-input-examples": [
          "Send an message to 'test@example.com' including a joke. Please use emojis inside it.",
          "Send an email to 'test@example.com' including a joke. Please use emojis inside it.",
          "Tell to 'test@example.com' that his new car is available.",
          "Write a profesional email to reject the candidate 'John Doe <test@example.com'"
        ]
      }
    }
  },
  [...]
```

You have a configuration example here: `./configs/api.sendgrid.com.oas.json`

## Add the API to tyk configuration

Your OAS API is ready to be integrated on the Tyk plugin:

```bash
curl http://localhost:8080/tyk/apis/oas \
  --header "x-tyk-authorization: foo" \
  --header 'Content-Type: text/plain' \
  -d@configs/api.sendgrid.com.oas.json

curl http://localhost:8080/tyk/reload/group \
  --header "x-tyk-authorization: foo"
```

## Test it !

Replace "agntcy@example.com" with a sender email you have configured on your sendgrid account.

```bash
curl http://localhost:8080/sendgrid/ \
  --header "Authorization: Bearer $SENDGRID_API_KEY" \
  --header 'Content-Type: application/nlq' \
  -d 'Send a message from me (agntcy@example.com) to John Die <j.doe@example.com>. John is french, the message should be a joke using a lot of emojis, something fun about comparing France and Italy'
```

As a result, the receiver (j.doe@example.com) must receive a mail like:
```

Subject: A Little Joke for You! 🇫🇷🇮🇹

Hey John! 😄

I hope you're having a fantastic day! I just wanted to share a little joke with you:

Why did the French chef break up with the Italian chef? 🍽️❤️

Because he couldn't handle all the pasta-bilities! 🍝😂

But don't worry, they still have a "bready" good friendship! 🥖😜

Just remember, whether it's croissants or cannoli, we can all agree that food brings us together! 🍷🍰

Take care and keep smiling! 😊

Best,
agntcy

```
