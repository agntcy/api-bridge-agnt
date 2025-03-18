// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"text/template"

	"github.com/TykTechnologies/tyk/ctx"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

var (
	noOverrideHeaders = []string{"Authorization"}

	tmplQuerySystemPrompt    *template.Template
	tmplQueryUserPrompt      *template.Template
	tmplResponseSystemPrompt *template.Template
	tmplResponseUserPrompt   *template.Template

	structuredOASResponse []byte = []byte("{}") // Will be updated in init()
)

// Struct given when rendering the templates
type TmplPromptOpenAPI struct {
	Operation string // The OpenAPI operation
	Sentence  string // The Natural Language sentence
}

type TmplPromptResponse struct {
	ResponseBody string // The response body
	UserRequest  string // The user request
}

func shouldRewriteQuery(r *http.Request) bool {
	enabled_header := trimAndLower(r.Header.Get(HEADER_X_NL_QUERY_ENABLED))
	if enabled_header == "" {
		return false
	}

	// We only want to rewrite the query if the content type is not set or is text/plain
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if contentType != "" && contentType != "text/plain" {
		logger.Debugf("[+] We were asked to rewrite the query but the Content-Type is not text/plain, ignoring ...")
		return false
	}

	return isEnabled(enabled_header)
}

func getRoute(req *http.Request) (*routers.Route, map[string]string, error) {
	oasDef := getOASDefinition(req)
	if oasDef == nil {
		logger.Errorf("[+] No OAS definition found in the request")
		return nil, nil, errors.New("no OAS definition found in the request")
	}

	// In order to find the route we need to strip the listenPath from the URL
	listenPath := oasDef.GetTykExtension().Server.ListenPath.Value
	strippedPath := stripListenPath(listenPath, req.URL.Path)
	oasDef.Servers = openapi3.Servers{}
	fakeReq := &http.Request{
		Method: req.Method,
		URL:    &url.URL{Path: strippedPath},
	}

	router, _ := gorillamux.NewRouter(&oasDef.T)
	route, pathParams, err := router.FindRoute(fakeReq)
	if err != nil {
		logger.Errorf("[+] Error finding route %s %s: %s", req.Method, strippedPath, err)
		return nil, nil, err
	}

	return route, pathParams, nil
}

func rewriteQuery(r *http.Request) error {
	route, pathParams, err := getRoute(r)
	if err != nil {
		logger.Errorf("[+] Error getting the route: %s", err)
		return errors.New("i'm sorry but I was not able to find the service you are asking for")
	}

	return rewriteQueryForRoute(r, route, pathParams)
}

func rewriteQueryForRoute(r *http.Request, route *routers.Route, pathParams map[string]string) error {
	config, err := initPluginFromRequest(r)
	if err != nil {
		return fmt.Errorf("can't retreive the LLM configuration: %w", err)
	}

	nlSentence, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("[+] Error while reading the body: %s", err)
		return errors.New("i'm sorry but I was not able to understand your query")
	}
	newParams := llmNlToOpenAPIRequest(r.Context(), route.Operation, string(nlSentence), config.LlmConfig)
	if newParams == nil {
		logger.Errorf("[+] Error creating the new request")
		return errors.New("I'm sorry but I was not able to understand your query")
	}
	// Override the method
	r.Method = route.Method

	// Override headers
	for hName, hValues := range newParams.InHeaderParams {
		if slices.Contains(noOverrideHeaders, hName) {
			continue
		}
		r.Header.Del(hName)
		for _, hValue := range hValues {
			r.Header.Add(hName, hValue)
		}
	}

	// Override query parameters
	queryParams := r.URL.Query()
	for qName, qValues := range newParams.InQueryParams {
		queryParams.Del(qName)
		for _, qValue := range qValues {
			queryParams.Add(qName, qValue)
		}
	}
	r.URL.RawQuery = queryParams.Encode()

	// Add new path parameters
	for k, v := range newParams.InPathParams {
		r.URL.Path = strings.Replace(r.URL.Path, "{"+k+"}", v, -1)
	}

	// Add the original path parameters if they are not in the new path parameters
	for k, v := range pathParams {
		r.URL.Path = strings.Replace(r.URL.Path, "{"+k+"}", v, -1)
	}

	// Override the body
	if newParams.RequestBody != "" {
		r.Body = io.NopCloser(strings.NewReader(newParams.RequestBody))
		r.ContentLength = int64(len(newParams.RequestBody))
	} else {
		r.Body = nil
		r.ContentLength = 0
	}
	r.Header.Del("Content-Length")

	return nil
}

func getOriginalNLQuery(r *http.Request) string {
	session := ctx.GetSession(r)
	if session == nil {
		return ""
	}

	nlQuery := session.MetaData["NLQuery"].(string)
	return nlQuery
}

func shouldRewriteResponseToNl(r *http.Request) bool {
	session := ctx.GetSession(r)
	if session == nil {
		return false
	}

	response_type := trimAndLower(session.MetaData[METADATA_RESPONSE_TYPE].(string))

	return response_type == RESPONSE_TYPE_NL
}

func trimAndLower(s string) string {
	return strings.Trim(strings.ToLower(s), " ")
}

func isEnabled(s string) bool {
	ENABLED_VALUES := []string{"true", "yes", "1", "ok"}
	return slices.Contains(ENABLED_VALUES, strings.ToLower(s))
}

func stripListenPath(listenPath string, path string) string {
	if listenPath == "" {
		return path
	}

	path = strings.TrimPrefix(path, listenPath)

	if path[0] != '/' {
		path = "/" + path
	}

	return path
}

func getSchemasRef(schemas []*openapi3.SchemaRef, refs map[string]*openapi3.Schema) {
	for _, schema := range schemas {
		if schema == nil {
			return
		}

		if schema.Ref != "" {
			if _, exists := refs[schema.Ref]; exists {
				continue
			}
			refs[schema.Ref] = schema.Value
		}

		if schema.Value.AnyOf != nil {
			getSchemasRef(schema.Value.AnyOf, refs)
		} else if schema.Value.OneOf != nil {
			getSchemasRef(schema.Value.OneOf, refs)
		} else if schema.Value.AllOf != nil {
			getSchemasRef(schema.Value.AllOf, refs)
		} else if schema.Value.Not != nil {
			getSchemasRef([]*openapi3.SchemaRef{schema.Value.Not}, refs)
		} else if schema.Value.Items != nil {
			getSchemasRef([]*openapi3.SchemaRef{schema.Value.Items}, refs)
		} else if schema.Value.Properties != nil {
			for _, prop := range schema.Value.Properties {
				getSchemasRef([]*openapi3.SchemaRef{prop}, refs)
			}
		}
	}
}

func getRequestBodyRefs(requestBody *openapi3.MediaType, refs map[string]*openapi3.Schema) {
	if requestBody == nil {
		return
	}

	getSchemasRef([]*openapi3.SchemaRef{requestBody.Schema}, refs)
}

func getParameterRefs(parameters openapi3.Parameters, refs map[string]*openapi3.Schema) {
	for _, p := range parameters {
		if p.Value != nil && p.Value.Schema != nil {
			getSchemasRef([]*openapi3.SchemaRef{p.Value.Schema}, refs)
		}
	}
}

// Here we are building a string representation of the operation
// It contains the list of dereferenced parameters and the list of references used in the operation
func buildOperationString(operation *openapi3.Operation, mediaType *openapi3.MediaType) (string, error) {
	refs := map[string]*openapi3.Schema{}

	var sb strings.Builder

	if operation.Description != "" {
		fmt.Fprintf(&sb, "Operation description: %s\n", operation.Description)
	} else if operation.Summary != "" {
		fmt.Fprintf(&sb, "Operation summary: %s\n", operation.Summary)
	}

	if len(operation.Parameters) > 0 {
		getParameterRefs(operation.Parameters, refs)

		sb.WriteString("The list of Parameters:\n")
		for _, param := range operation.Parameters {
			if param == nil || param.Value == nil {
				continue
			}
			m, err := param.Value.MarshalJSON()
			if err != nil {
				logger.Errorf("[+] Error marshalling parameter: %s", err)
				return "", err
			}
			fmt.Fprintf(&sb, "- %s\n", string(m))
		}
	}

	if operation.RequestBody != nil {
		getRequestBodyRefs(mediaType, refs)
		sb.WriteString("The request body:\n")
		m, err := mediaType.Schema.Value.MarshalJSON()
		if err != nil {
			logger.Errorf("[+] Error marshalling request body: %s", err)
			return "", err
		}
		fmt.Fprintf(&sb, "%s\n", string(m))
	}

	if len(refs) > 0 {
		sb.WriteString("The list of References:\n")

		// Sort refs
		sortedRefs := make([]string, 0, len(refs))
		for refName := range refs {
			sortedRefs = append(sortedRefs, refName)
		}
		sort.Strings(sortedRefs)

		for _, refName := range sortedRefs {
			ref := refs[refName]
			m, err := ref.MarshalJSON()
			if err != nil {
				logger.Errorf("[+] Error marshalling reference: %s", err)
				return "", err
			}
			fmt.Fprintf(&sb, "- %s: %s\n", refName, string(m))
		}
	}

	return sb.String(), nil
}

func selectRequestBodyMediaType(requestBody *openapi3.RequestBody) (string, *openapi3.MediaType) {
	if len(requestBody.Content) == 0 {
		return "", nil
	}

	// Return the only content type
	if len(requestBody.Content) == 1 {
		for k, v := range requestBody.Content {
			return k, v
		}
	}

	preferedMediaTypes := []string{"application/json"}
	// Prefered media types over other media types
	for _, ct := range preferedMediaTypes {
		mt := requestBody.GetMediaType(ct)
		if mt != nil {
			return ct, mt
		}
	}

	return "", nil
}

// Let's convert that back to a JSON object
type openAPIOperationParams struct {
	InPathParams   map[string]string `json:"in_path_params"`
	InQueryParams  url.Values        `json:"in_query_params"`
	InHeaderParams http.Header       `json:"in_header_params"`
	RequestBody    string            `json:"request_body"`
}

func llmNlToOpenAPIRequest(context context.Context, operation *openapi3.Operation, nlSentence string, llmConfig *NLAPIConfig) *openAPIOperationParams {
	mediaTypeStr := ""
	var md *openapi3.MediaType
	if operation.RequestBody != nil {
		mediaTypeStr, md = selectRequestBodyMediaType(operation.RequestBody.Value)
	}
	operationString, err := buildOperationString(operation, md)
	if err != nil {
		logger.Errorf("[+] Error while building operation string: %s", err)
		return nil
	}

	systemPromptBuf := new(bytes.Buffer)
	err = tmplQuerySystemPrompt.Execute(systemPromptBuf, TmplPromptOpenAPI{Operation: string(operationString), Sentence: nlSentence})
	if err != nil {
		logger.Errorf("[+] Error while creating the System prompt: %s", err)
		return nil
	}

	userPromptBuf := new(bytes.Buffer)
	err = tmplQueryUserPrompt.Execute(userPromptBuf, TmplPromptOpenAPI{Operation: string(operationString), Sentence: nlSentence})
	if err != nil {
		logger.Errorf("[+] Error while creating the User prompt: %s", err)
		return nil
	}

	operationTool := JsonSchemaResponse{
		Name:        "convert_to_openapi",
		Description: "",
		Schema:      structuredOASResponse,
	}
	translation, err := llmCall(context, systemPromptBuf.String(), userPromptBuf.String(), &operationTool, llmConfig)
	if err != nil {
		logger.Errorf("[+] Error translating text: %s", err)
		return nil
	}
	logger.Debugf("[+] Translation: %s\n", translation)

	llmOperation := openAPIOperationParams{}
	err = json.Unmarshal([]byte(translation), &llmOperation)
	if err != nil {
		logger.Errorf("[+] Error while unmarshalling the JSON object: %s", err)
		return nil
	}

	llmOperation.InHeaderParams.Del("Content-Encoding")
	llmOperation.InHeaderParams.Set("Content-Type", mediaTypeStr)

	return &llmOperation
}

type JsonSchemaResponse struct {
	Name        string
	Description string
	Schema      []byte
}

func llmCall(ctx context.Context, systemPrompt string, data string, schemaResponse *JsonSchemaResponse, llmConfig *NLAPIConfig) (string, error) {
	logger.Debugf("[+] Generated system prompt: %s", systemPrompt)
	logger.Debugf("[+] Generated user prompt: %s", data)

	chatCompletions := azopenai.ChatCompletionsOptions{
		Messages: []azopenai.ChatRequestMessageClassification{
			&azopenai.ChatRequestAssistantMessage{
				Content: azopenai.NewChatRequestAssistantMessageContent(systemPrompt),
			},
			&azopenai.ChatRequestUserMessage{
				Content: azopenai.NewChatRequestUserMessageContent(data),
			},
		},
		MaxTokens:      to.Ptr(int32(2048)),
		Temperature:    to.Ptr(float32(0.0)),
		Seed:           to.Ptr(int64(42)),
		DeploymentName: &llmConfig.AzureConfig.ModelDeployment,
	}

	if schemaResponse != nil {
		chatCompletions.ResponseFormat = &azopenai.ChatCompletionsJSONSchemaResponseFormat{
			JSONSchema: &azopenai.ChatCompletionsJSONSchemaResponseFormatJSONSchema{
				Name:        &schemaResponse.Name,
				Description: &schemaResponse.Description,
				Schema:      schemaResponse.Schema,
				Strict:      to.Ptr(false),
			},
		}
	}
	resp, err := llmConfig.azureClient.GetChatCompletions(ctx, chatCompletions, nil)
	if err != nil {
		logger.Errorf("[+] Error translating text: %s", err)
		return "", err
	}

	if len(resp.Choices) > 0 && resp.Choices[0].Message != nil && resp.Choices[0].Message.Content != nil {
		return *resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("unable to get a response from the LLM")
}

func responseToNL(r *http.Request, upstreamResponse string) (string, error) {
	var err error

	originalQuery := getOriginalNLQuery(r)

	systemPromptBuf := new(bytes.Buffer)
	err = tmplResponseSystemPrompt.Execute(systemPromptBuf, TmplPromptResponse{ResponseBody: upstreamResponse, UserRequest: originalQuery})
	if err != nil {
		return "", fmt.Errorf("error while creating the system prompt: %w", err)
	}

	userPromptBuf := new(bytes.Buffer)
	err = tmplResponseUserPrompt.Execute(userPromptBuf, TmplPromptResponse{ResponseBody: upstreamResponse, UserRequest: originalQuery})
	if err != nil {
		return "", fmt.Errorf("error while creating the user prompt: %w", err)
	}

	config, err := initPluginFromRequest(r)
	if err != nil {
		return "", fmt.Errorf("can't retreive the LLM configuration: %w", err)
	}

	translation, err := llmCall(r.Context(), systemPromptBuf.String(), userPromptBuf.String(), nil, config.LlmConfig)
	if err != nil {
		return "", fmt.Errorf("error translating text: %w", err)
	}

	return translation, nil
}

func initQueryTemplates() {
	var err error

	systemPrompt := `
You are given an OpenAPI operation description with:
- a list of parameters defined as OpenAPI parameters,
- an optional request body described as OpenAPI MediaType object,
- a list of references ($ref) if used in the parameters or the the request body.

Your goal is to convert the natural language sentence to a JSON object following the OpenAPI operation description.
- Only use required parameters,
- You MUST use the exact name of the parameters.
- DO NOT invent. If information is missing, DO NOT include it.

The OpenAPI operation description:
====
{{.Operation}}
====`

	userPrompt := `The natural language sentence:
====
{{.Sentence}}
====`

	tmplQuerySystemPrompt, err = template.New("system_prompt_convert_to_openapi").Parse(systemPrompt)
	if err != nil {
		logger.Fatalf("[+] Error parsing the system prompt template: %s", err)
	}
	tmplQueryUserPrompt, err = template.New("user_prompt_convert_to_openapi").Parse(userPrompt)
	if err != nil {
		logger.Fatalf("[+] Error parsing the user prompt template: %s", err)
	}
}

func initResponseTemplates() {
	var err error

	systemPrompt := `Given a API reponse body, and an instruction from a user. You must convert it to a natural language text, according to the user's request.

The API response body:
====
{{.ResponseBody}}
====
`

	userPrompt := `The user's request:

====
{{.UserRequest}}
====
`
	tmplResponseSystemPrompt, err = template.New("system_prompt_convert_to_nl").Parse(systemPrompt)
	if err != nil {
		logger.Fatalf("[+] Error parsing the system prompt template: %s", err)
	}

	tmplResponseUserPrompt, err = template.New("user_prompt_convert_to_nl").Parse(userPrompt)
	if err != nil {
		logger.Fatalf("[+] Error parsing the user prompt template: %s", err)
	}
}

func initStructuredOasResponse() {
	structuredOASResponse = []byte(`{
"type": "object",
"properties": {
  "in_path_params": {
    "description": "The parameters that are inside the path",
    "type": "object",
    "additionalProperties": { "type": "string" }
  },
  "in_query_params": {
    "description": "The parameters that are part of the query string. Each parameter is an array of strings",
    "type": "object",
    "additionalProperties": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "in_header_params": {
    "description": "The parameters that are in the headers. Each parameter is an array of strings",
    "type": "object",
    "additionalProperties": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "request_body": {
    "description": "The optional content of the body",
    "type": "string"
  }
},
"required": ["in_path_params", "in_query_params", "in_header_params", "request_body"],
"additionalProperties": false
}`)
}

func init() {
	initStructuredOasResponse()
	initQueryTemplates()
	initResponseTemplates()
}
