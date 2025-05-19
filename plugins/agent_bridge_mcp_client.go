// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	DEFAULT_MAX_LLM_ITERATIONS = 3
	DEFAULT_LLM_SEED           = 42
	DEFAULT_LLM_TEMPERATURE    = 0.0
)

type MCPServers map[string]*MCPServerConfig

type TykMCPConfig struct {
	MCPServers   MCPServers      `json:"mcpServers"`
	MCPLLMConfig MCPOpenAIConfig `json:"openai,omitempty"`
}

type MCPServerConfig struct {
	Name    string   `json:"name"`
	SSE     string   `json:"sse,omitempty"` // Either SSE or Command
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`

	Client client.MCPClient
	Tools  []mcp.Tool
}

var mcpConfig MCPServers = MCPServers{}

type MCPOpenAIConfig struct {
	OpenAIKey       string `json:"openAIKey"`
	OpenAIEndpoint  string `json:"openAIEndpoint"`
	ModelDeployment string `json:"modelDeployment"`
}

type MCPLLMConfig struct {
	openAIConfig MCPOpenAIConfig
	azureClient  *azopenai.Client
}

var llmConfig = MCPLLMConfig{}

func ProcessMCPQuery(rw http.ResponseWriter, r *http.Request) {
	logger.Debugf("[+] Inside ProcessMCPQuery ...")

	if r.URL.Path == "/mcp/init" || len(mcpConfig) == 0 {
		deinitMCPClient()
		mcpConfig = MCPServers{}
		err := loadMCPPluginConfig(r)
		if err != nil {
			logger.Errorf("[+] Failed to load MCP plugin config: %s", err)
			http.Error(rw, "Error while loading MCP configuration", http.StatusInternalServerError)
			return
		}
		err = initMCPClient()
		if err != nil {
			logger.Errorf("[+] Failed to initialize MCP servers: %s", err)
			http.Error(rw, "Error while loading MCP sub-system", http.StatusInternalServerError)
			return
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("MCP Server(s) Initialized"))
		return
	}

	// POST and Content-Type: application/nlq are expected
	if r.URL.Path != "/mcp/" || r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusNotFound)
		return
	} else if !isNLQContentType(r.Header.Get("Content-Type")) {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(rw, "You must use POST /mcp/ with Content-Type: %s", CONTENT_TYPE_NLQ)
		return
	}

	nlqBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("[+] Error while reading the body: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}
	nlq := string(nlqBytes)
	logger.Debugf("[+] Process query: %v", nlq)
	response, err := processQueryWithMCP(nlq)
	if err != nil {
		logger.Errorf("[+] Failed to process query: %s, err=%s", nlq, err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(response))
}

func callMCPTool(toolName string, args *string) (string, error) {
	// The arguments for the function is provided as a JSON string
	funcParams := map[string]any{}
	if args != nil {
		err := json.Unmarshal([]byte(*args), &funcParams)
		if err != nil {
			logger.Errorf("[+] Failed to unmarshal function parameters: %v", err)
			return "", err
		}
	}

	var mcpServerWithTool *MCPServerConfig = nil
	for _, mcpServer := range mcpConfig {
		for _, tool := range mcpServer.Tools {
			if tool.Name == toolName {
				mcpServerWithTool = mcpServer
				break
			}
		}
	}

	if mcpServerWithTool == nil {
		logger.Errorf("[+] Failed to find MCP server for tool: %s", toolName)
		return "", fmt.Errorf("unable to find tool '%s'", toolName)
	}

	logger.Infof("[+] Calling tool: (%s) from server (%s)", toolName, mcpServerWithTool.Name)

	toolRequest := mcp.CallToolRequest{}
	toolRequest.Params.Name = toolName
	toolRequest.Params.Arguments = funcParams

	callResult, err := mcpServerWithTool.Client.CallTool(context.TODO(), toolRequest)
	if err != nil {
		logger.Errorf("[+] Failed to call tool: %v", err)
		return "", err
	}

	result := ""
	for _, content := range callResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			result = result + textContent.Text
		} else {
			jsonBytes, err := json.MarshalIndent(content, "", "  ")
			if err != nil {
				logger.Warn("[+] MCP result is not of format TextContent, nor JSON serializable, ignoring ...")
				continue
			}
			result = result + string(jsonBytes)
		}
	}

	logger.Debugf("[+] Tool call result: %+v\n", result)
	return result, nil
}

func processQueryWithMCP(nlq string) (string, error) {
	logger.Debugf("[+] processQueryWithMCP('%s') ...", nlq)

	// Create the list of all available tools
	availableTools := []mcp.Tool{}
	for _, c := range mcpConfig {
		availableTools = append(availableTools, c.Tools...)
	}

	if len(availableTools) == 0 {
		logger.Errorf("[+] processQueryWithMCP('%s') no available tools", nlq)
		return "", fmt.Errorf("no available tools")
	}
	if llmConfig.azureClient == nil {
		logger.Error("[+] No LLM configured in MCP configuration")
		return "", fmt.Errorf("no LLM configured in MCP configuration")
	}

	// Ask to the LLM
	messages := []azopenai.ChatRequestMessageClassification{
		&azopenai.ChatRequestUserMessage{
			Content: azopenai.NewChatRequestUserMessageContent(nlq),
		},
	}
	llmTools := []azopenai.ChatCompletionsToolDefinitionClassification{}
	for _, tool := range availableTools {
		functionDefinition, err := getChatCompletionFunctionDefinition(tool)
		if err != nil {
			logger.Errorf("[+] processQueryWithMCP('%s') failed to get function definition for tool (%s): %v", nlq, tool.Name, err)
			continue
		}
		llmTools = append(llmTools, &azopenai.ChatCompletionsFunctionToolDefinition{
			Function: functionDefinition,
		},
		)
	}

	round := 0
	for round < DEFAULT_MAX_LLM_ITERATIONS {
		round++

		resp, err := llmConfig.azureClient.GetChatCompletions(context.TODO(), azopenai.ChatCompletionsOptions{
			DeploymentName: &llmConfig.openAIConfig.ModelDeployment,
			Messages:       messages,
			Tools:          llmTools,
			Temperature:    to.Ptr[float32](DEFAULT_LLM_TEMPERATURE),
			Seed:           to.Ptr[int64](DEFAULT_LLM_SEED),
		}, nil)
		if err != nil {
			logger.Errorf("[+] Failed to query LLM: %s", err)
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no choices in LLM response")
		}

		choice := resp.Choices[0]
		if choice.FinishReason == nil {
			return "", fmt.Errorf("LLM returned a choice with no finish reason")
		}

		messages = append(messages, &azopenai.ChatRequestAssistantMessage{
			ToolCalls: choice.Message.ToolCalls,
		})

		if *choice.FinishReason == azopenai.CompletionsFinishReasonStopped {
			if choice.Message == nil || choice.Message.Content == nil {
				logger.Errorf("[+] ERROR: no content in the response")
				return "", fmt.Errorf("no content in the response")
			}
			logger.Debugf("[+] Stop is detected, final response is (%v) in %v round", *choice.Message.Content, round)
			return *choice.Message.Content, nil
		}
		for _, toolCall := range choice.Message.ToolCalls {
			functionToolCall, ok := toolCall.(*azopenai.ChatCompletionsFunctionToolCall)
			if !ok {
				logger.Errorf("[+] Unexpected error, something is wrong in the azure-sdk-for-go library, ignoring ...")
				continue
			}
			result, err := callMCPTool(*functionToolCall.Function.Name, functionToolCall.Function.Arguments)
			if err != nil {
				logger.Errorf("[+] Failed to call tool (%s): %v", *functionToolCall.Function.Name, err)
				messages = append(messages, &azopenai.ChatRequestToolMessage{
					Content:    azopenai.NewChatRequestToolMessageContent("An error occurred while calling the tool"),
					ToolCallID: functionToolCall.ID,
				})
				continue
			}

			messages = append(messages, &azopenai.ChatRequestToolMessage{
				Content:    azopenai.NewChatRequestToolMessageContent(result),
				ToolCallID: functionToolCall.ID,
			})
		}

	}

	// We reached the limit of rounds
	return "", fmt.Errorf("reached the limit of rounds")
}

func getChatCompletionFunctionDefinition(tool mcp.Tool) (*azopenai.ChatCompletionsFunctionToolDefinitionFunction, error) {
	jsonBytes, err := json.Marshal(tool.InputSchema)
	if err != nil {
		logger.Fatalf("[+] Failed to marshal parameters of tool (%v): err=%v", tool, err)
	}

	// Create the function definition
	functionDefinition := &azopenai.ChatCompletionsFunctionToolDefinitionFunction{
		Name:        &tool.Name,
		Description: &tool.Description,
		Parameters:  jsonBytes,
	}
	return functionDefinition, nil
}

func deinitMCPClient() {
	for _, config := range mcpConfig {
		if config.Client != nil {
			config.Client.Close()
			config.Client = nil
		}
	}
}

func initMCPClient() error {
	for name, config := range mcpConfig {
		if config.Client != nil {
			continue // already initialized
		}
		logger.Debugf("[+] initMCPClient(%s) ...", name)

		// If the env configuration value contains ${VAR} (like
		// 'GITHUB_PERSONAL_ACCESS_TOKEN=${GITHUB_PERSONAL_ACCESS_TOKEN}'), let's
		// take the value from the environment variable.
		for index, env := range config.Env {
			tokens := strings.SplitN(env, "=", 2)
			if len(tokens) == 2 && len(tokens[1]) > 0 && strings.HasPrefix(tokens[1], "${") && strings.HasSuffix(tokens[1], "}") {
				varName := tokens[1][2 : len(tokens[1])-1]
				envValue := os.Getenv(varName)
				if envValue == "" {
					logger.Errorf("[+] Environment variable %s not found", varName)
					return fmt.Errorf("MCP configuration error")
				}
				config.Env[index] = fmt.Sprintf("%s=%s", tokens[0], envValue)
			}
		}

		if config.SSE != "" {
			logger.Infof("[+] initMCPClient(%s): Using SSE transport to %s\n", name, config.SSE)

			client, err := client.NewSSEMCPClient(config.SSE)
			if err != nil {
				logger.Errorf("[+] Failed to create client: %v", err)
				return fmt.Errorf("MCP configuration error")
			}

			err = client.Start(context.TODO())
			if err != nil {
				logger.Fatalf("[+] Failed to start client: %v", err)
				return fmt.Errorf("MCP configuration error")
			}
			config.Client = client
		} else if config.Command != "" {
			logger.Infof("[+] initMCPClient(%s): Using stdio transport for command %s\n", name, config.Command)

			client, err := client.NewStdioMCPClient(config.Command, config.Env, config.Args...)
			if err != nil {
				logger.Errorf("[+] Failed to create client: %v", err)
				return fmt.Errorf("MCP configuration error")
			}
			config.Client = client
		} else {
			logger.Errorf("[+] Either 'sse' or 'command' must be provided for the MCP Server configuration")
			return fmt.Errorf("MCP configuration error")
		}

		logger.Infof("[+] Initializing client")
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "api-bridge-agent",
			Version: "1.0.0",
		}
		initResult, err := config.Client.Initialize(context.TODO(), initRequest)
		if err != nil {
			logger.Errorf("[+] Failed to initialize: %v", err)
			return fmt.Errorf("MCP Initialization error")
		}
		logger.Infof(
			"[+] Initialized with server: %s %s\n\n",
			initResult.ServerInfo.Name,
			initResult.ServerInfo.Version,
		)

		logger.Infof("[+] Listing available tools...")
		toolsRequest := mcp.ListToolsRequest{}
		tools, err := config.Client.ListTools(context.TODO(), toolsRequest)
		if err != nil {
			logger.Errorf("[+] Failed to list tools: %v", err)
			return fmt.Errorf("MCP initialization error")
		}
		config.Tools = tools.Tools
		for _, tool := range tools.Tools {
			logger.Infof("[+]   - %s: %s\n", tool.Name, tool.Description)
		}
	}

	return nil
}

func loadMCPPluginConfig(r *http.Request) error {
	logger.Debugf("[+] loadMCPPluginConfig ...")

	apidef := getOASDefinition(r)
	middleware := apidef.GetTykMiddleware()
	if middleware == nil {
		err := fmt.Errorf("Tyk middleware definition is nil")
		logger.Errorf("[+] Error while loading MCP configuration: %s", err)
		return err
	}
	globalPluginConfig := middleware.Global.PluginConfig
	if globalPluginConfig == nil {
		err := fmt.Errorf("Tyk global.pluginConfig definition is nil")
		logger.Errorf("[+] Error while loading MCP configuration: %s", err)
		return err
	}
	if globalPluginConfig.Data == nil {
		err := fmt.Errorf("Tyk global.pluginConfig.data definition is nil")
		logger.Errorf("[+] Error while loading MCP configuration: %s", err)
		return err
	}

	configValue, err := json.Marshal(globalPluginConfig.Data.Value)
	if err != nil {
		logger.Errorf("[+] Error while loading MCP configuration: %s", err)
		return err
	}

	mcpTykConfig := TykMCPConfig{}
	err = json.Unmarshal([]byte(configValue), &mcpTykConfig)
	if err != nil {
		logger.Errorf("[+] Error while loading MCP configuration: %s", err)
		return err
	}
	for name := range mcpTykConfig.MCPServers {
		mcpTykConfig.MCPServers[name].Name = name
	}
	mcpConfig = mcpTykConfig.MCPServers

	llmConfig.openAIConfig = mcpTykConfig.MCPLLMConfig
	llmConfig.openAIConfig.OpenAIEndpoint = getEnvOrDefault(llmConfig.openAIConfig.OpenAIEndpoint, "OPENAI_ENDPOINT", DEFAULT_OPENAI_ENDPOINT)
	llmConfig.openAIConfig.OpenAIKey = getEnvOrDefault(llmConfig.openAIConfig.OpenAIKey, "OPENAI_API_KEY", "")
	llmConfig.openAIConfig.ModelDeployment = getEnvOrDefault(llmConfig.openAIConfig.ModelDeployment, "OPENAI_MODEL", DEFAULT_OPENAI_MODEL)

	if llmConfig.openAIConfig.OpenAIKey == "" {
		err := fmt.Errorf("missing required OpenAI Key. Either set OPENAI_API_KEY environement variable or set the 'openai.openAIKey' configuration")
		logger.Errorf("[+] Error initializing plugin: %s", err)
		return err
	}

	// Note: eventually cache these by hash of config?
	keyCredential := azcore.NewKeyCredential(llmConfig.openAIConfig.OpenAIKey)
	if llmConfig.openAIConfig.OpenAIEndpoint == DEFAULT_OPENAI_ENDPOINT {
		llmConfig.azureClient, err = azopenai.NewClientForOpenAI(llmConfig.openAIConfig.OpenAIEndpoint, keyCredential, nil)
	} else {
		llmConfig.azureClient, err = azopenai.NewClientWithKeyCredential(llmConfig.openAIConfig.OpenAIEndpoint, keyCredential, nil)
	}
	if err != nil {
		logger.Errorf("[+] Unable to create OpenAI client: %s", err)
		return err
	}

	return nil
}
