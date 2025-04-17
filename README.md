# API Bridge Agnt

- [Documentation](https://docs.agntcy.org/pages/syntactic_sdk/api_bridge_agent.html)

## About The Project

The API Bridge Agnt project provides a [Tyk](https://tyk.io/) middleware plugin
that allows users to interact with traditional REST APIs using natural language.
It acts as a translator between human language and structured API
requests/responses.

Key features:
- Converts natural language queries into valid API requests based on OpenAPI specifications
- Transforms API responses back into natural language explanations
- Integrates with Tyk API Gateway as a plugin
- Uses Azure OpenAI's GPT models for language processing
- Preserves API schema validation and security while enabling conversational interfaces

This enables developers to build more accessible and user-friendly API interfaces without modifying
the underlying API implementations.


## Getting Started

Build the plugin and start Tyk Gateway:
```bash
export OPENAI_API_KEY=REPLACE_WITH_YOUR_KEY
export OPENAI_MODEL=gpt-4o-mini

make start_tyk   # This will automatically build Tyk and the plugin, then install the plugin and start Tyk gateway
```

Add an example API (github.com) to Tyk:
```bash
curl http://localhost:8080/tyk/apis/oas \
  --header "x-tyk-authorization: foo" \
  --header 'Content-Type: text/plain' \
  -d@configs/api.github.com.gist.deref.oas.json

curl http://localhost:8080/tyk/reload/group \
  --header "x-tyk-authorization: foo"
```

Test the plugin with a natural language query:
```bash
curl 'http://localhost:8080/github/' \
  --header 'Content-Type: application/nlq' \
  -d 'List the first issue for the repository named tyk owned by TykTechnologies with the label bug'
```

## Plugin Configuration

You can tune the operation matching sensitivity per API by setting the `relevanceThreshold` (a float between 0 and 1, default is 0.5) in your Tyk pluginConfig. A higher value requires a stronger semantic match.

Example plugin configuration snippet in your API definition:
```json
[...]
  "middleware": {
    "global": {
      "pluginConfig": {
        "data": {
          "enabled": true,
          "value": {
            "azureConfig": {
              "openAIKey": "YOUR_OPENAI_KEY",
              "modelDeployment": "gpt-4o-mini"
            },
            "selectOperations": {
              "getIssue": {
                "x-nl-input-examples": [
                  "List the first issue for the repository named tyk owned by TykTechnologies with the label bug"
                ]
              }
            },
            "relevanceThreshold": 0.7
          }
        }
      }
    }
  }
[...]
```

## Contributing

Contributions are what make the open source community such an amazing place to
learn, inspire, and create. Any contributions you make are **greatly
appreciated**. For detailed contributing guidelines, please see
[CONTRIBUTING.md](CONTRIBUTING.md)

## Copyright Notice

[Copyright Notice and License](./LICENSE)

Distributed under Apache 2.0 License. See LICENSE for more information.
Copyright AGNTCY Contributors (https://github.com/agntcy)
