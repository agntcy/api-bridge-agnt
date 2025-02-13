# API Bridge Agnt

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

### Prerequisites

- Go
- Cmake
- Git
- jq

### Local development

Environment variables (OpenAI):

```
export OPENAI_API_KEY=REPLACE_WITH_YOUR_KEY
export OPENAI_MODEL=gpt-4o-mini
```

Environment variables (Azure OpenAI):

```
export OPENAI_API_KEY=REPLACE_WITH_YOUR_KEY
export OPENAI_ENDPOINT=https://REPLACE_WITH_YOUR_ENDPOINT.openai.azure.com
export OPENAI_MODEL=gpt-4o-mini
```

Dependencies are managed so that you can also just run: `make start_tyk` and the
plugin will be built and installed first.

Quick start:
```bash
make start_tyk   # This will automatically build Tyk and the plugin, then install the plugin and start Tyk gateway
make load_plugin # This will configure Tyk with an example API (httpbin.org) and reload the gateway
```

Individual steps if needed:
```bash
make build_tyk
make build_plugin
make install_plugin
```

## Usage

### Tyk configuration

In the Tyk Gateway configuration file (`tyk.conf`), add the plugin to the
`postPlugins` and `responsePlugins` sections:

```json
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
```

### Select and rewrite middleware

The first middleware function (`SelectAndRewrite`) is responsible for selecting
the appropriate OpenAPI endpoint based on the request, and then rewriting the
request to match the expected API format.

The content type for this request should be `application/nlq`.

Example:
```bash
curl 'http://localhost:8080/github/' \
  --header 'Content-Type: application/nlq' \
  -d 'List the first issue for the repository named tyk owned by TykTechnologies with the label bug'
```

### Rewrite query

The second middleware function (`RewriteQueryToOas`) is only responsible for
converting the natural language query into a valid API request based on the
selected OpenAPI endpoint.

!> Here you MUST provide the full path of the target API in the request URL.

Two headers are available for this request:

- **X-Nl-Query-Enabled**: `yes` or `no` (default is `no`), to enable or disable the natural language query processing
- **X-Nl-Response-Type**: `nl` or `upstream` (default is `upstream`), to select the response format. `nl` will return the response in natural language, while `upstream` will return the original API response.

Example:
```bash
curl 'http://localhost:8080/gmail/gmail/v1/users/me/messages/send' \
  --header "Authorization: Bearer YOUR_GOOGLE_TOKEN" \
  --header 'Content-Type: text/plain' \
  --header 'X-Nl-Query-Enabled: yes' \
  --header 'X-Nl-Response-Type: nl' \
  --data 'Send an email to "john.doe@example.com". Explain that we are accepting is offer for Agntcy'
```

### Rewrite response

The third middleware function (`RewriteResponseToNl`) is responsible for
converting the API response into natural language.
It can be used standalone or in combination with the `RewriteQueryToOas` middleware.

### How to add an API to Tyk

1. Add OAS spec:

```bash
curl http://localhost:8080/tyk/apis/oas \
  --header "x-tyk-authorization: foo" \
  --header 'Content-Type: text/plain' \
  -d@configs/httpbin.org.oas.json

curl http://localhost:8080/tyk/reload/group \
  --header "x-tyk-authorization: foo"
```

2. Test request:

```bash
curl http://localhost:8080/httpbin/json \
  --header "X-Nl-Query-Enabled: yes" \
  --header "X-Nl-Response-Type: nl" \
  --header "Content-Type: text/plain" \
  -d "Hello"
```

### Github example

```bash
curl 'http://localhost:8080/github/' \
  --header 'Content-Type: application/nlq' \
  -d 'List the first issue for the repository named tyk owned by TykTechnologies with the label bug'
```

### Sendgrid example

```bash
curl http://localhost:8080/sendgrid/v3/mail/send \
  --header "Authorization: Bearer $SENDGRID_API_KEY" \
  --header 'Content-Type: application/nlq' \
  -d 'Send a message from me (agntcy@example.com) to John Die <j.doe@example.com>. John is french, the message should be a joke using a lot of emojis, something fun about comparing France and Italy'
```


## Contributing

Contributions are what make the open source community such an amazing place to
learn, inspire, and create. Any contributions you make are **greatly
appreciated**. For detailed contributing guidelines, please see
[CONTRIBUTING.md](CONTRIBUTING.md)

## Copyright Notice

[Copyright Notice and License](./LICENSE.md)

Copyright (c) 2025 Cisco and/or its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
