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

Tyk requires also a redis database. You can deploy it with:

```bash
make start_redis
```

### Local development

Built with:

- [search](https://github.com/kelindar/search) for the semantic router
- [Tyk](https://github.com/TykTechnologies/tyk.git) for the gateway
We use these dependencies inside the project. However, you don't need to download it or to build it, 
everything is managed by the Makefile.

#### Step 1 - Set environment variables

For OpenAI:

```bash
export OPENAI_API_KEY=REPLACE_WITH_YOUR_KEY
export OPENAI_MODEL=gpt-4o-mini
```

For Azure OpenAI:

```bash
export OPENAI_API_KEY=REPLACE_WITH_YOUR_KEY
export OPENAI_ENDPOINT=https://REPLACE_WITH_YOUR_ENDPOINT.openai.azure.com
export OPENAI_MODEL=gpt-4o-mini
```

#### Step 2 - Build the plugin and start tyk locally on [Tyk](http://localhost:8080)

Dependencies are managed so that you can just run:

```bash
make start_tyk
```

This will automatically build "Tyk", "search" and the plugin, then install the plugin and start Tyk gateway

#### Step 3 - Load and configure Tyk with an example API (httpbin.org)

```bash
make load_plugin
```

### Other installation

#### Linux

For linux (ubuntu) you can use:

```bash
TARGET_OS=linux TARGET_ARCH=amd64 SEARCH_LIB=libllama_go.so make start_tyk
```

#### Individual steps for building if needed:

If you need to decompose each task individually, you can split into

```bash
make build_tyk          # build tyk
make build_search_lib   # build the "search" library, used as semantic router
make build_plugin       # build the plugin
make install_plugin     # Install the plugin
```

## Tyk configuration

This plugins relies on [Tyk OAS API Definition](https://tyk.io/docs/api-management/gateway-config-tyk-oas/).
To use it, you need to add the plugin to the `postPlugins` and `responsePlugins`
sections of the `x-tyk-api-gateway` section:

```json
"x-tyk-api-gateway": {
[...]
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
[...]
    }
  }
}
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
!> Rewriting the query will be available only if the content type is not set or is text/plain

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

In this example "http://localhost:8080/gmail/gmail/v1/users/me/messages/send":

- "/gmail/" is the listen path defined on the x-tyk-api-gateway part of the spec
- "gmail/v1/users/me/messages/send" is the endpoint on the spec

### Rewrite response

The third middleware function (`RewriteResponseToNl`) is responsible for
converting the API response into natural language.
It can be used standalone or in combination with the `RewriteQueryToOas` middleware.

### How to add an API to Tyk

1. Add OAS spec:

Adding a spec named "httpbin.org.oas.json" located in ./configs folder

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

In this example "http://localhost:8080/httpbin/json":

- "/httpbin/" is the listen path defined on the x-tyk-api-gateway part of the spec
- "json" is the endpoint on the spec

### Github example

```bash
curl 'http://localhost:8080/github/' \
  --header 'Content-Type: application/nlq' \
  -d 'List the first issue for the repository named tyk owned by TykTechnologies with the label bug'
```

## Example Usage

### [Using Sendgrid Mail API](./using_sendgrid_api.md)

### [Adding a new API from scratch](./add_api_opensky.md)

## Troobleshooting

```
error obtaining VCS status: exit status 128
	Use -buildvcs=false to disable VCS stamping.
```

Ether disable VCS control, or
git config --global --add safe.directory <root_folder>/api-bridge-agnt/tyk-release-v5.8.0-alpha7

## Contributing

Contributions are what make the open source community such an amazing place to
learn, inspire, and create. Any contributions you make are **greatly
appreciated**. For detailed contributing guidelines, please see
[CONTRIBUTING.md](./CONTRIBUTING.md)

## Copyright Notice

[Copyright Notice and License](https://github.com/agntcy/api-bridge-agnt/blob/main/LICENSE)

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
