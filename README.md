# API Bridge Agnt

- [Documentation](https://agntcy.github.io/api-bridge-agnt)

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

Add an example API (httpbin.org) to Tyk:
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

## Contributing

Contributions are what make the open source community such an amazing place to
learn, inspire, and create. Any contributions you make are **greatly
appreciated**. For detailed contributing guidelines, please see
[CONTRIBUTING.md](CONTRIBUTING.md)

## Copyright Notice

[Copyright Notice and License](LICENSE.md)

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
