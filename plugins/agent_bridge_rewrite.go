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
	oasDef := ctx.GetOASDefinition(req)
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
	var ENABLED_VALUES = []string{"true", "yes", "1", "ok"}
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

func pruneOperation(operation openapi3.Operation) *openapi3.Operation {
	// Here we try to reduce the size of the operation that will be sent to the LLM
	newOperation := &openapi3.Operation{
		Parameters:  operation.Parameters,
		RequestBody: operation.RequestBody,
		Summary:     operation.Summary,
	}

	// Prefer the description over the summary
	if operation.Description != "" {
		newOperation.Description = operation.Description
	} else if operation.Summary != "" {
		newOperation.Description = operation.Summary
	}

	return newOperation
}

// Let's convert that back to a JSON object
type openAPIOperationParams struct {
	InPathParams   map[string]string `json:"in_path_params"`
	InQueryParams  url.Values        `json:"in_query_params"`
	InHeaderParams http.Header       `json:"in_header_params"`
	RequestBody    string            `json:"request_body"`
}

func llmNlToOpenAPIRequest(context context.Context, operation *openapi3.Operation, nlSentence string, llmConfig *NLAPIConfig) *openAPIOperationParams {
	lightOperation := pruneOperation(*operation)
	operationJSON, err := lightOperation.MarshalJSON()
	if err != nil {
		logger.Errorf("[+] Error marshalling operation: %s", err)
		return nil
	}

	systemPromptBuf := new(bytes.Buffer)
	err = tmplQuerySystemPrompt.Execute(systemPromptBuf, TmplPromptOpenAPI{Operation: string(operationJSON), Sentence: nlSentence})
	if err != nil {
		logger.Errorf("[+] Error while creating the System prompt: %s", err)
		return nil
	}

	userPromptBuf := new(bytes.Buffer)
	err = tmplQueryUserPrompt.Execute(userPromptBuf, TmplPromptOpenAPI{Operation: string(operationJSON), Sentence: nlSentence})
	if err != nil {
		logger.Errorf("[+] Error while creating the User prompt: %s", err)
		return nil
	}

	operationTool := JsonSchemaResponse{
		Name:        "convert_to_openapi",
		Description: "Convert a natural language sentence to a JSON object following the OpenAPI operation schema",
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
				Strict:      to.Ptr(true),
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

	systemPrompt := `Given an OpenAPI specification operation you convert the natural language sentence to a JSON object following the OpenAPI operation schema.
You MUST use the exact name of the parameters. DO NOT invent. If information is missing, DO NOT include it. Do not forget to include the content type in the headers.

The OpenAPI operation specification:
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
	var err error
	structuredOASResponse, err = json.Marshal(map[string]any{
		"type":        "object",
		"description": "Represents the parameters and the body of an OpenAPI operation",
		"properties": map[string]any{
			"in_path_params": map[string]any{
				"description": "The parameters that are inside the path (in: path)",
				"type":        "object",
				"additionalProperties": map[string]any{
					"type": "string",
				},
			},
			"in_query_params": map[string]any{
				"description": "The parameters that are part of the query string (in: query)",
				"type":        "object",
				"additionalProperties": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"in_header_params": map[string]any{
				"description": "The parameters that are in the headers (in: header)",
				"type":        "object",
				"additionalProperties": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"request_body": map[string]any{
				"description": "The optional content of the body",
				"type":        "string",
			},
		},
		// FIXME: required doesn't seem to work well, why ?
		// "required":             []string{"in_query_params", "in_path_params", "in_header_params", "request_body"},
		"required":             []string{"request_body"},
		"additionalProperties": false,
	})
	if err != nil {
		logger.Fatalf("[+] Error while creating the structured oas response object. It's certainly a bug: %s", err)
	}
}

func init() {
	initStructuredOasResponse()
	initQueryTemplates()
	initResponseTemplates()
}
