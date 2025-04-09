// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Client  *client.Client
	Name    string `json:"name"`
}

var mcpConfig = map[string]*MCPServerConfig{}

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
	// POST and Content-Type: application/nlq are expected
	if !(r.Method == "POST" && r.Header.Get("Content-Type") == CONTENT_TYPE_NLQ) {
		logger.Debugf("[+] Query is not POST or Content-Type is not %s, ignoring ...", CONTENT_TYPE_NLQ)
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
	logger.Debugf("[+] Found a response (%v) for query=%v", response, nlq)

	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(response))
}

func processQueryWithMCP(nlq string) (string, error) {
	logger.Debugf("[+] processQueryWithMCP('%s') ...", nlq)

	// Create the list of all available tools
	availableTools := []*protocol.Tool{}
	for name, item := range mcpConfig {
		if item.Client != nil {
			logger.Errorf("[+] processQueryWithMCP('%s') using mcpClient (%s)", nlq, name)
			toolsResult, err := item.Client.ListTools(context.Background())
			if err != nil {
				log.Fatalf("[+] Failed to list tools for server (%s): %v", name, err)
			}
			availableTools = append(availableTools, toolsResult.Tools...)
		} else {
			logger.Errorf("[+] processQueryWithMCP('%s') mcpClient is nil for server (%s)", nlq, name)
		}
	}
	dump("[+] Available tools: %+v\n", availableTools)

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
	}, nil)
	if err != nil {
		logger.Errorf("[+] ERROR: %s", err)
		return "", err
	}
	logger.Info("[+] -------------------------")
	dump("[+] LLM response: ", resp)

	if len(resp.Choices) > 0 && *resp.Choices[0].FinishReason == azopenai.CompletionsFinishReasonToolCalls {
		// Add the tool call message from the response to the messages. It allow to the LLM to match the response with the tool call
		//messages = append(messages, resp.Choices[0].Message)
		messages = append(messages, &azopenai.ChatRequestAssistantMessage{
			ToolCalls: resp.Choices[0].Message.ToolCalls,
		})
		// Then process the tool calls
		for _, toolCall := range resp.Choices[0].Message.ToolCalls {
			dump("[+] Tool call: ", toolCall)
			funcCall := toolCall.(*azopenai.ChatCompletionsFunctionToolCall).Function
			// Call the tool
			for name, item := range mcpConfig {
				if item.Client == nil {
					continue
				}
				toolsResult, err := item.Client.ListTools(context.Background())
				for _, tool := range toolsResult.Tools {
					if tool.Name == *funcCall.Name {
						logger.Infof("[+] Calling tool: (%s) from server (%s)", tool.Name, name)
						// The arguments for the function come back as a JSON string
						funcParams := map[string]any{}
						err = json.Unmarshal([]byte(*funcCall.Arguments), &funcParams)
						if err != nil {
							logger.Errorf("[+] Failed to unmarshal function parameters: %v", err)
							continue
						}
						callResult, err := item.Client.CallTool(context.Background(), protocol.NewCallToolRequest(*funcCall.Name, funcParams))
						if err != nil {
							logger.Errorf("[+] Failed to call tool (%s): %v", tool.Name, err)
							continue
						}
						b, _ := json.Marshal(callResult)
						logger.Infof("[+] Tool call result: %+v\n", string(b))
						messages = append(messages, &azopenai.ChatRequestToolMessage{
							Content:    azopenai.NewChatRequestToolMessageContent(string(b)),
							ToolCallID: toolCall.(*azopenai.ChatCompletionsFunctionToolCall).ID,
						})
					}
				}
			}
		}
		logger.Info("[+] -------------------------")
		dump("[+] messages: ", messages)
		// Ask to the LLM again with the tool call result
		resp, err := llmConfig.azureClient.GetChatCompletions(context.TODO(), azopenai.ChatCompletionsOptions{
			DeploymentName: &llmConfig.azureConfig.ModelDeployment,
			Messages:       messages,
			Tools:          llmTools,
			Temperature:    to.Ptr[float32](0.0),
		}, nil)
		if err != nil {
			logger.Errorf("[+] ERROR: %s", err)
			return "", err
		}
		logger.Info("[+] -------------------------")
		dump("[+] LLM Final response: ", resp)
		if len(resp.Choices) > 0 && resp.Choices[0].Message != nil && resp.Choices[0].Message.Content != nil {
			return *resp.Choices[0].Message.Content, nil
		}

	}
	/*
		if mcpClient == nil {
			return "", fmt.Errorf("mcpClient is nil")
		}

		// List available tools
		toolsResult, err := mcpClient.ListTools(context.Background())
		if err != nil {
			log.Fatalf("[+] Failed to list tools: %v", err)
		}
		b, _ := json.Marshal(toolsResult.Tools)
		logger.Infof("[+] Available tools: %+v\n", string(b))

		// Call tool
		callResult, err := mcpClient.CallTool(
			context.Background(),
			protocol.NewCallToolRequest("current time", map[string]interface{}{
				"timezone": "UTC",
			}))
		if err != nil {
			log.Fatalf("[+] Failed to call tool: %v", err)
		}
		b, _ = json.Marshal(callResult)
		logger.Infof("[+] Tool call result: %+v\n", string(b))
		return string(b), nil
	*/
	return "", nil
}

func getChatCompletionFunctionDefinition(tool *protocol.Tool) (*azopenai.ChatCompletionsFunctionToolDefinitionFunction, error) {
	jsonBytes, err := json.Marshal(tool.InputSchema)
	if err != nil {
		log.Fatalf("[+] Failed to marshal parameters of tool (%v): err=%v", tool, err)
	}

	// Create the function definition
	functionDefinition := &azopenai.ChatCompletionsFunctionToolDefinitionFunction{
		Name:        &tool.Name,
		Description: &tool.Description,
		Parameters:  jsonBytes,
	}
	return functionDefinition, nil
}

func initMCPClient() {
	for name, config := range mcpConfig {
		if config.Client != nil {
			continue // already initialized
		}
		logger.Debugf("[+] initMCPClient(%s) ...", name)
		if config.Command != "" {
			logger.Errorf("[+] initMCPClient(%s) stdio not supported for now, skipping", name)
			continue
		}
		if config.Args == nil || len(config.Args) == 0 {
			logger.Errorf("[+] initMCPClient(%s) args is empty, skipping", name)
			continue
		}
		// Create transport client (using SSE in this example)
		logger.Infof("[+] initMCPClient(%s): Using SSE transport to %s\n", name, config.Args[0])
		transportClient, err := transport.NewSSEClientTransport(config.Args[0])
		if err != nil {
			log.Fatalf("[+] Failed to create transport client: %v", err)
		}

		// Create MCP client using transport
		config.Client, err = client.NewClient(transportClient, client.WithClientInfo(protocol.Implementation{
			Name:    name,
			Version: "1.0.0",
		}))
		if err != nil {
			log.Fatalf("[+] Failed to create MCP client: %v", err)
		}
		//defer mcpClient.Close()

		// List available tools
		toolsResult, err := config.Client.ListTools(context.Background())
		if err != nil {
			log.Fatalf("[+] Failed to list tools: %v", err)
		}
		b, _ := json.Marshal(toolsResult.Tools)
		logger.Infof("[+] Available tools: %+v\n", string(b))

		// Call tool
		/*callResult, err := config.Client.CallTool(
			context.Background(),
			protocol.NewCallToolRequest("current time", map[string]interface{}{
				"timezone": "UTC",
			}))
		if err != nil {
			log.Fatalf("[+] Failed to call tool: %v", err)
		}
		b, _ = json.Marshal(callResult)
		logger.Infof("[+] Tool call result: %+v\n", string(b))*/
	}
}

func loadMCPPluginConfig(r *http.Request) error {
	logger.Debugf("[+] loadMCPPluginConfig ...")
	configValue := `{
		"current-time-v2-server": {
			"command": "",
			"args": ["http://127.0.0.1:8088/sse"]
		},
		"weather": {
			"command": "python",
			"args": ["../weather-server-python/weather.py"]
		},
		"git": {
			"command": "python",
			"args": ["../servers/git/src/mcp_server_git/server.py", "--repository", "/media/sf_vmshared/api-bridge-agnt"]
		},
		"github": {
			"command": "docker",
			"args": [
				"run",
				"-i",
				"--rm",
				"-e",
				"GITHUB_PERSONAL_ACCESS_TOKEN",
				"ghcr.io/github/github-mcp-server"
			],
			"env": {
				"GITHUB_PERSONAL_ACCESS_TOKEN": ""
			}
		}
	}`
	err := json.Unmarshal([]byte(configValue), &mcpConfig)
	if err != nil {
		logger.Fatalf("[+] conversion error for acpPluginConfig: %s", err)
	}
	dump("[+] mcpConfig :", mcpConfig)

	llmConfigData := map[string]any{}
	llmConfig.azureConfig = MCPAzureConfig{
		OpenAIEndpoint:  getConfigValue(DEFAULT_OPENAI_ENDPOINT, llmConfigData, "openAIEndpoint", "OPENAI_ENDPOINT"),
		OpenAIKey:       getConfigValue("", llmConfigData, "openAIKey", "AZURE_OPENAI_API_KEY"),
		ModelDeployment: getConfigValue(DEFAULT_OPENAI_MODEL, llmConfigData, "modelDeployment", "OPENAI_MODEL_DEPLOYMENT_ID"),
	}

	if llmConfig.azureConfig.OpenAIKey == "" {
		err := fmt.Errorf("Missing required config for azureConfig.openAIKey")
		logger.Fatalf("[+] Error initializing plugin: %s", err)
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
		logger.Fatalf("[+] Unable to create OpenAI client: %s", err)
		return err
	}
	dump("[+] azureConfig: ", llmConfig.azureConfig)

	return nil
}

func init() {
	logger.Infof("[+] Initializing API Bridge Agnt plugin ...")
	loadMCPPluginConfig(nil)
	initMCPClient()
}
