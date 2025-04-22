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
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const MAX_RESULT = 3

type MCPServers map[string]*MCPServerConfig

type TykMCPConfig struct {
	MCPServers MCPServers `json:"mcpServers"`
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

type MCPAzureConfig struct {
	OpenAIKey       string `json:"openAIKey"`
	OpenAIEndpoint  string `json:"openAIEndpoint"`
	ModelDeployment string `json:"modelDeployment"`
}

type MCPLLMConfig struct {
	azureConfig MCPAzureConfig
	azureClient *azopenai.Client
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
	if r.URL.Path != "/mcp/" || r.Method != http.MethodPost || !isNLQContentType(r.Header.Get("Content-Type")) {
		logger.Debugf("[+] Query is not POST or Content-Type is not %s, ignoring ...", CONTENT_TYPE_NLQ)
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte(fmt.Sprintf("You must use POST / with Content-Type: %s", CONTENT_TYPE_NLQ)))
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

func processQueryWithMCP(nlq string) (string, error) {
	logger.Debugf("[+] processQueryWithMCP('%s') ...", nlq)

	// Create the list of all available tools
	availableTools := []mcp.Tool{}
	for _, c := range mcpConfig {
		availableTools = append(availableTools, c.Tools...)
	}
	logger.Infof("[+] Nb Available tools: %+v\n", len(availableTools))

	if len(availableTools) == 0 {
		logger.Errorf("[+] processQueryWithMCP('%s') no available tools", nlq)
		return "", fmt.Errorf("no available tools")
	}
	if llmConfig.azureClient == nil {
		logger.Errorf("[+] processQueryWithMCP('%s') azureClient is nil", nlq)
		return "", fmt.Errorf("azureClient is nil")
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
	resp, err := llmConfig.azureClient.GetChatCompletions(context.TODO(), azopenai.ChatCompletionsOptions{
		DeploymentName: &llmConfig.azureConfig.ModelDeployment,
		Messages:       messages,
		Tools:          llmTools,
		Temperature:    to.Ptr[float32](0.0),
		Seed:           to.Ptr[int64](42),
	}, nil)
	if err != nil {
		logger.Errorf("[+] ERROR: %s", err)
		return "", err
	}
	// If the LLM answered directly (no tool calls), return that content immediately
	if len(resp.Choices) > 0 && *resp.Choices[0].FinishReason != azopenai.CompletionsFinishReasonToolCalls {
		if resp.Choices[0].Message != nil && resp.Choices[0].Message.Content != nil {
			return *resp.Choices[0].Message.Content, nil
		}
		return "", fmt.Errorf("no content in LLM response")
	}

	round := 0
	logger.Info("[+] -------------------------")

	for len(resp.Choices) > 0 && *resp.Choices[0].FinishReason == azopenai.CompletionsFinishReasonToolCalls && round < MAX_RESULT {
		round++
		// Add the tool call message from the response to the messages. It allow to the LLM to match the response with the tool call
		// messages = append(messages, resp.Choices[0].Message)
		messages = append(messages, &azopenai.ChatRequestAssistantMessage{
			ToolCalls: resp.Choices[0].Message.ToolCalls,
		})
		// Then process the tool calls
		for _, toolCall := range resp.Choices[0].Message.ToolCalls {
			funcCall := toolCall.(*azopenai.ChatCompletionsFunctionToolCall).Function
			// Call the tool
			calledTool := []string{}
			for name, item := range mcpConfig {
				for _, tool := range item.Tools {
					if slices.Contains(calledTool, tool.Name) {
						continue
					}
					if tool.Name == *funcCall.Name {
						logger.Infof("[+] Calling tool: (%s) from server (%s)", tool.Name, name)
						// The arguments for the function come back as a JSON string
						funcParams := map[string]any{}
						err = json.Unmarshal([]byte(*funcCall.Arguments), &funcParams)
						if err != nil {
							logger.Errorf("[+] Failed to unmarshal function parameters: %v", err)
							continue
						}

						logger.Println("Doing tool request")
						listTmpRequest := mcp.CallToolRequest{}
						listTmpRequest.Params.Name = tool.Name
						listTmpRequest.Params.Arguments = funcParams

						callResult, err := item.Client.CallTool(context.TODO(), listTmpRequest)
						if err != nil {
							logger.Errorf("[+] Failed to call tool (%s): %v", tool.Name, err)
							messages = append(messages, &azopenai.ChatRequestToolMessage{
								Content:    azopenai.NewChatRequestToolMessageContent("An error occurred while calling the tool"),
								ToolCallID: toolCall.(*azopenai.ChatCompletionsFunctionToolCall).ID,
							})
							continue
						}

						result := ""

						for _, content := range callResult.Content {
							if textContent, ok := content.(mcp.TextContent); ok {
								result = result + textContent.Text
							} else {
								jsonBytes, _ := json.MarshalIndent(content, "", "  ")
								result = result + string(jsonBytes)
							}
						}

						logger.Infof("[+] Tool call result: %+v\n", result)
						messages = append(messages, &azopenai.ChatRequestToolMessage{
							Content:    azopenai.NewChatRequestToolMessageContent(result),
							ToolCallID: toolCall.(*azopenai.ChatCompletionsFunctionToolCall).ID,
						})

						calledTool = append(calledTool, tool.Name)
					}
				}
			}
		}
		// Ask to the LLM again with the tool call result
		resp, err = llmConfig.azureClient.GetChatCompletions(context.TODO(), azopenai.ChatCompletionsOptions{
			DeploymentName: &llmConfig.azureConfig.ModelDeployment,
			Messages:       messages,
			Tools:          llmTools,
			Temperature:    to.Ptr[float32](0.0),
			Seed:           to.Ptr[int64](42),
		}, nil)
		if err != nil {
			logger.Errorf("[+] ERROR: %s", err)
			return "", err
		}

		logger.Info("[+] -------------------------")
		if len(resp.Choices) > 0 && *resp.Choices[0].FinishReason == azopenai.CompletionsFinishReasonStopped {
			if resp.Choices[0].Message == nil || resp.Choices[0].Message.Content == nil {
				logger.Errorf("[+] ERROR: no content in the response")
				return "", fmt.Errorf("no content in the response")
			}
			logger.Infof("[+] Stop is detected, final response is (%v) in %v round", *resp.Choices[0].Message.Content, round)
			return *resp.Choices[0].Message.Content, nil
		}

	}

	return "", nil
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

		// If the env configuration value start with '$' (like
		// 'GITHUB_PERSONAL_ACCESS_TOKEN=$GITHUB_PERSONAL_ACCESS_TOKEN'), let's
		// take the value from the environment variable.
		for index, env := range config.Env {
			tokens := strings.Split(env, "=")
			if len(tokens) == 2 && len(tokens[1]) > 0 && tokens[1][0] == '$' {
				envValue := os.Getenv(tokens[1][1:])
				if envValue == "" {
					logger.Errorf("Environment variable %s not found", tokens[1][1:])
					return fmt.Errorf("MCP configuration error")
				}
				config.Env[index] = fmt.Sprintf("%s=%s", tokens[0], envValue)
			}
		}

		if config.SSE != "" {
			logger.Infof("[+] initMCPClient(%s): Using SSE transport to %s\n", name, config.SSE)

			client, err := client.NewSSEMCPClient(config.SSE)
			if err != nil {
				logger.Errorf("Failed to create client: %v", err)
				return fmt.Errorf("MCP configuration error")
			}

			err = client.Start(context.TODO())
			if err != nil {
				logger.Fatalf("Failed to start client: %v", err)
				return fmt.Errorf("MCP configuration error")
			}
			config.Client = client
		} else if config.Command != "" {
			logger.Infof("[+] initMCPClient(%s): Using stdio transport for command %s\n", name, config.Command)

			client, err := client.NewStdioMCPClient(config.Command, config.Env, config.Args...)
			if err != nil {
				logger.Errorf("Failed to create client: %v", err)
				return fmt.Errorf("MCP configuration error")
			}
			config.Client = client
		} else {
			logger.Errorf("Either 'sse' or 'command' must be provided for the MCP Server configuration")
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
			logger.Errorf("Failed to initialize: %v", err)
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
			logger.Errorf("Failed to list tools: %v", err)
			return fmt.Errorf("MCP initialization error")
		}
		config.Tools = tools.Tools
		for _, tool := range tools.Tools {
			logger.Infof("   - %s: %s\n", tool.Name, tool.Description)
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
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return err
	}
	globalPluginConfig := middleware.Global.PluginConfig
	if globalPluginConfig == nil {
		err := fmt.Errorf("Tyk global.pluginConfig definition is nil")
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return err
	}
	if globalPluginConfig.Data == nil {
		err := fmt.Errorf("Tyk global.pluginConfig.data definition is nil")
		logger.Errorf("[+] Error while loading MCP configuration: %s", err)
		return err
	}

	configValue, err := json.Marshal(globalPluginConfig.Data.Value)
	if err != nil {
		logger.Errorf("[+] Invalid MCP Configuration: %s", err)
		return err
	}

	mcpTykConfig := TykMCPConfig{}
	err = json.Unmarshal([]byte(configValue), &mcpTykConfig)
	if err != nil {
		logger.Errorf("[+] conversion error for acpPluginConfig: %s", err)
		return err
	}
	mcpConfig = mcpTykConfig.MCPServers

	llmConfigData := map[string]any{}
	llmConfig.azureConfig = MCPAzureConfig{
		OpenAIEndpoint:  getConfigValue(DEFAULT_OPENAI_ENDPOINT, llmConfigData, "openAIEndpoint", "OPENAI_ENDPOINT"),
		OpenAIKey:       getConfigValue("", llmConfigData, "openAIKey", "AZURE_OPENAI_API_KEY"),
		ModelDeployment: getConfigValue(DEFAULT_OPENAI_MODEL, llmConfigData, "modelDeployment", "OPENAI_MODEL_DEPLOYMENT_ID"),
	}

	if llmConfig.azureConfig.OpenAIKey == "" {
		err := fmt.Errorf("Missing required config for azureConfig.openAIKey")
		logger.Errorf("[+] Error initializing plugin: %s", err)
		return err
	}

	// Note: eventually cache these by hash of config?
	keyCredential := azcore.NewKeyCredential(llmConfig.azureConfig.OpenAIKey)
	if llmConfig.azureConfig.OpenAIEndpoint == DEFAULT_OPENAI_ENDPOINT {
		llmConfig.azureClient, err = azopenai.NewClientForOpenAI(llmConfig.azureConfig.OpenAIEndpoint, keyCredential, nil)
	} else {
		llmConfig.azureClient, err = azopenai.NewClientWithKeyCredential(llmConfig.azureConfig.OpenAIEndpoint, keyCredential, nil)
	}
	if err != nil {
		logger.Errorf("[+] Unable to create OpenAI client: %s", err)
		return err
	}

	return nil
}
