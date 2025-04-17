// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/TykTechnologies/tyk/apidef/oas"
	"github.com/kelindar/search"
)

const (
	SPEC_EXT_AI_INPUT_EXAMPLES = "x-nl-input-examples"

	DEFAULT_MODEL_EMBEDDINGS_PATH  = "models"
	DEFAULT_MODEL_EMBEDDINGS_MODEL = "jina-embeddings-v2-base-en-q5_k_m.gguf"
	DEFAULT_OPENAI_ENDPOINT        = "https://api.openai.com/v1"
	DEFAULT_OPENAI_MODEL           = "gpt-4o-mini"

	MAX_UTERANCE_LENGTH   = 1500
	VECTORIZER_GPU_LAYERS = 1
)

type AzureConfig struct {
	OpenAIKey       string `json:"openAIKey"`
	OpenAIEndpoint  string `json:"openAIEndpoint"`
	ModelDeployment string `json:"modelDeployment"`
}

type NLAPIConfig struct {
	AzureConfig AzureConfig
	azureClient *azopenai.Client
}

var embeddingModels = map[string]*search.Vectorizer{} // model name -> vectorizer
var embeddingModelsLock = &sync.RWMutex{}
var apiSpecIndices = map[string]*search.Index[string]{} // apiId -> indices for ops in API spec
var apiSpecIndicesLock = &sync.RWMutex{}

// PluginConfig is only supported at the API definition level.
var pluginConfig = map[string]*PluginDataConfig{} // api id -> config
var pluginConfigLock = &sync.RWMutex{}

type AIExtensionConfig struct {
	InputExamples []string `json:"x-nl-input-examples"`
}

type PluginDataConfig struct {
	AzureConfig          AzureConfig                   `json:"azureConfig"`
	SelectOperations     map[string]*AIExtensionConfig `json:"selectOperations"`
	SelectModelEmbedding string                        `json:"selectModelEmbedding"`
	SelectModelsPath     string                        `json:"selectModelsPath"`
	LlmConfig            *NLAPIConfig                  `json:"llmConfig"`
	// RelevanceThreshold is the minimum matching score to select an operation; default is 0.5
	RelevanceThreshold float64 `json:"relevanceThreshold,omitempty"`

	APIID      string
	ListenPath string
}

func getApiId(r *http.Request) (string, error) {
	apidef := getOASDefinition(r)

	if apidef == nil {
		// TOOD: fallback on classic...
		return "", fmt.Errorf("API definition is nil")
	}
	gateway := apidef.GetTykExtension()
	if gateway == nil {
		return "", fmt.Errorf("the Tyk gateway definition is nil")
	}
	return gateway.Info.ID, nil
}

func getConfigValue(defaultValue string, configData map[string]any, configMapKey string, envValue string) string {
	ret := defaultValue
	v, exists := configData[configMapKey]
	if exists {
		ret = v.(string)
	}
	if envValue != "" && os.Getenv(envValue) != "" {
		ret = os.Getenv(envValue)
	}

	return ret
}

func parseConfigData(apiId string, configData map[string]any) (*PluginDataConfig, error) {
	logger.Debugf("[+] Parsing config for api id: %s", apiId)

	azureConfigData, exists := configData["azureConfig"].(map[string]any)
	if !exists {
		azureConfigData = map[string]any{}
	}

	// Determine relevance threshold (default or overridden)
	threshold := DEFAULT_RELEVANCE_THRESHOLD
	if v, exists := configData["relevanceThreshold"]; exists {
		if f, ok := v.(float64); ok {
			threshold = f
		} else {
			logger.Warningf("[+] Invalid type for relevanceThreshold: %T; using default %f", v, threshold)
		}
	}
	pluginDataConfig := &PluginDataConfig{
		AzureConfig: AzureConfig{
			OpenAIEndpoint:  getConfigValue(DEFAULT_OPENAI_ENDPOINT, azureConfigData, "openAIEndpoint", "OPENAI_ENDPOINT"),
			OpenAIKey:       getConfigValue("", azureConfigData, "openAIKey", "OPENAI_API_KEY"),
			ModelDeployment: getConfigValue(DEFAULT_OPENAI_MODEL, azureConfigData, "modelDeployment", "OPENAI_MODEL"),
		},
		SelectOperations:     map[string]*AIExtensionConfig{},
		SelectModelEmbedding: DEFAULT_MODEL_EMBEDDINGS_MODEL,
		SelectModelsPath:     DEFAULT_MODEL_EMBEDDINGS_PATH,
		RelevanceThreshold:   threshold,

		APIID: apiId,
	}

	logger.Debugf("[+] Finished parsing config for api id: %s", apiId)
	return pluginDataConfig, nil
}

func initPluginFromRequest(apiId string, apiDef *oas.OAS) (*PluginDataConfig, error) {
	logger.Debugf("[+] Initializing for api id: %s", apiId)

	if apiDef == nil {
		err := fmt.Errorf("API definition is nil for api id: %s", apiId)
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return nil, err
	}

	middleware := apiDef.GetTykMiddleware()
	if middleware == nil {
		err := fmt.Errorf("the Tyk middleware definition is nil for api id: %s", apiId)
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return nil, err
	}
	globalPluginConfig := middleware.Global.PluginConfig
	if globalPluginConfig == nil {
		err := fmt.Errorf("the Tyk global.pluginConfig definition is nil for api id: %s", apiId)
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return nil, err
	}
	apiConfigData := globalPluginConfig.Data
	if apiConfigData == nil {
		err := fmt.Errorf("the Tyk pluginConfig.data definition is nil for api id: %s", apiId)
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return nil, err
	}

	pluginDataConfig, err := parseConfigData(apiId, apiConfigData.Value)
	if err != nil {
		logger.Fatalf("[+] Unable to parse configuration data: %s", err)
		return pluginDataConfig, err
	}

	if pluginDataConfig.AzureConfig.OpenAIKey == "" {
		err := fmt.Errorf("missing required config for azureConfig.openAIKey")
		logger.Fatalf("[+] Error initializing plugin: %s", err)
		return pluginDataConfig, err
	}

	// Iterate through all paths and operations in the API definition
	for _, path := range apiDef.Paths {
		for _, operation := range path.Operations() {
			// Check if this operation has AI input examples defined
			aiExamples, hasAiExamples := operation.Extensions[SPEC_EXT_AI_INPUT_EXAMPLES]
			if !hasAiExamples {
				continue
			}

			operationId := operation.OperationID
			aiExtentionConfig := pluginDataConfig.SelectOperations[operationId]
			if aiExtentionConfig == nil {
				aiExtentionConfig = &AIExtensionConfig{}
			}

			// Add each example to the operation's config
			for _, example := range aiExamples.([]any) {
				exampleStr, isString := example.(string)
				if !isString {
					logger.Fatalf("[+] Error parsing examples for operation %s", operationId)
				}
				aiExtentionConfig.InputExamples = append(aiExtentionConfig.InputExamples, exampleStr)
			}

			pluginDataConfig.SelectOperations[operationId] = aiExtentionConfig
		}
	}

	// If we have no operation with x-nl-input-examples then we rely only on the
	// descriptions
	if len(pluginDataConfig.SelectOperations) == 0 {
		for _, path := range apiDef.Paths {
			for _, operation := range path.Operations() {
				if operation.OperationID == "" {
					continue
				}
				aiExtentionConfig := &AIExtensionConfig{}
				aiExtentionConfig.InputExamples = append(aiExtentionConfig.InputExamples, operation.Description)
				aiExtentionConfig.InputExamples = append(aiExtentionConfig.InputExamples, operation.Summary)
				pluginDataConfig.SelectOperations[operation.OperationID] = aiExtentionConfig
			}
		}
	}

	// Note: eventually cache these by hash of config?
	keyCredential := azcore.NewKeyCredential(pluginDataConfig.AzureConfig.OpenAIKey)
	var client *azopenai.Client
	if pluginDataConfig.AzureConfig.OpenAIEndpoint == DEFAULT_OPENAI_ENDPOINT {
		client, err = azopenai.NewClientForOpenAI(pluginDataConfig.AzureConfig.OpenAIEndpoint, keyCredential, nil)
	} else {
		client, err = azopenai.NewClientWithKeyCredential(pluginDataConfig.AzureConfig.OpenAIEndpoint, keyCredential, nil)
	}
	if err != nil {
		logger.Fatalf("[+] Unable to create OpenAI client: %s", err)
		return pluginDataConfig, err
	}

	pluginDataConfig.LlmConfig = &NLAPIConfig{
		AzureConfig: pluginDataConfig.AzureConfig,
		azureClient: client,
	}

	if len(pluginDataConfig.SelectOperations) > 0 {
		// Note: create embedder before initializing indices!
		logger.Debugf("[+] Loading embedding model %s for api id: %s", pluginDataConfig.SelectModelEmbedding, apiId)
		embeddingModelsLock.RLock()
		_, present := embeddingModels[pluginDataConfig.SelectModelEmbedding]
		embeddingModelsLock.RUnlock()
		if present {
			logger.Debugf("[+] embedding model %s cached for api id: %s", pluginDataConfig.SelectModelEmbedding, apiId)
		} else {
			modelPath := filepath.Join(pluginDataConfig.SelectModelsPath, pluginDataConfig.SelectModelEmbedding)
			embeddingModelsLock.Lock()
			_, present := embeddingModels[pluginDataConfig.SelectModelEmbedding]
			if !present {
				modelEmbedder, err := search.NewVectorizer(modelPath, VECTORIZER_GPU_LAYERS)
				if err != nil {
					embeddingModelsLock.Unlock()
					logger.Fatalf("[+] Unable to find embedding model %s: %s", pluginDataConfig.SelectModelEmbedding, err)
					return pluginDataConfig, nil
				}
				embeddingModels[pluginDataConfig.SelectModelEmbedding] = modelEmbedder
			}
			embeddingModelsLock.Unlock()
			if !present {
				logger.Debugf("[+] Added embedding model %s for api id: %s", pluginDataConfig.SelectModelEmbedding, apiId)
			}
		}

		if err := initSelectOperations(apiId, pluginDataConfig); err != nil {
			logger.Fatalf("[+] failed to initialize select operations for api id %s: %s", apiId, err)
			return pluginDataConfig, err
		}
	}

	gateway := apiDef.GetTykExtension()
	if gateway == nil {
		return pluginDataConfig, fmt.Errorf("the Tyk gateway definition is nil")
	}
	pluginDataConfig.ListenPath = gateway.Server.ListenPath.Value

	// Save the plugin data config to the Redis store
	if err := saveApiUterances(apiId, pluginDataConfig); err != nil {
		logger.Fatalf("[+] failed to save plugin data config to redis store: %s", err)
		return pluginDataConfig, err
	}

	return pluginDataConfig, nil
}

func getPluginFromRequest(r *http.Request) (*PluginDataConfig, error) {
	apiId, err := getApiId(r)
	if err != nil {
		logger.Errorf("[+] getPluginFromRequest cannot find api id: %s", err)
		return nil, err
	}

	// Note: we really need to just to be able to clear the cache on API def
	// reloads to fix everything complicated.
	pluginConfigLock.RLock()
	pluginDataConfig, present := pluginConfig[apiId]
	pluginConfigLock.RUnlock()
	if present {
		logger.Debugf("[+] Config data already cached for api id: %s", apiId)
		return pluginDataConfig, nil
	}

	apiDef := getOASDefinition(r)
	// TOOD: fallback on classic...
	if apiDef == nil {
		err := fmt.Errorf("API definition is nil")
		logger.Errorf("[+] getPluginFromRequest: %s", err)
		return nil, err
	}

	pluginDataConfig, err = initPluginFromRequest(apiId, apiDef)
	if err != nil {
		logger.Fatalf("[+] Unable to parse configuration data: %s", err)
		return nil, err
	}

	pluginConfigLock.Lock()
	pluginConfig[apiId] = pluginDataConfig
	pluginConfigLock.Unlock()

	logConfig()

	logger.Debugf("[+] Finished getPluginFromRequest for api id: %s", apiId)
	return pluginDataConfig, nil
}

func initSelectOperations(apiId string, pluginDataConfig *PluginDataConfig) error {
	embeddingModelsLock.RLock()
	modelEmbedder, present := embeddingModels[pluginDataConfig.SelectModelEmbedding]
	embeddingModelsLock.RUnlock()
	if !present {
		return fmt.Errorf("no embedding model found for api id: %s", apiId)
	}

	apiSpecIndicesLock.RLock()
	_, present = apiSpecIndices[apiId]
	apiSpecIndicesLock.RUnlock()
	if present {
		logger.Debugf("[+] replacing index found for operations for api id: %s", apiId)
	}

	apiSpecIndex := search.NewIndex[string]()
	for apiOperation, aiExtension := range pluginDataConfig.SelectOperations {
		for _, example := range aiExtension.InputExamples {
			if len(example) > MAX_UTERANCE_LENGTH {
				logger.Warningf("[+] example too long: %s", example)
				continue
			}
			embedding, err := modelEmbedder.EmbedText(example)
			if err != nil {
				logger.Warningf("[+] embedding model %s failed for text \"%s\": %s", pluginDataConfig.SelectModelEmbedding, example, err)
			} else {
				apiSpecIndex.Add(embedding, apiOperation) // map embedding to operation name
			}
		}
	}

	// Always recreate. This is obviously a race condition on the loading of specs, but that should
	// be handled at a higher level than this plugin.
	apiSpecIndicesLock.Lock()
	apiSpecIndices[apiId] = apiSpecIndex
	apiSpecIndicesLock.Unlock()
	return nil
}

func logConfig() {
	pluginConfigLock.RLock()
	defer pluginConfigLock.RUnlock()

	for apiId, pluginDataConfig := range pluginConfig {
		logger.Infof("[+] Config %s: Azure OpenAI API Key: %s", apiId, "**REDACTED**")
		logger.Infof("[+] Config %s: Azure OpenAI Endpoint: %s", apiId, pluginDataConfig.AzureConfig.OpenAIEndpoint)
		logger.Infof("[+] Config %s: Azure OpenAI Model Deployment ID: %s", apiId, pluginDataConfig.AzureConfig.ModelDeployment)
		if len(pluginDataConfig.SelectOperations) > 0 {
			logger.Infof("[+] Config %s: Select operations: %d", apiId, len(pluginDataConfig.SelectOperations))
			logger.Infof("[+] Config %s: Select embedding model: %s", apiId, filepath.Join(pluginDataConfig.SelectModelsPath, pluginDataConfig.SelectModelEmbedding))
		}
	}
}

func deletePluginConfig(apiId string) {
	pluginConfigLock.Lock()
	defer pluginConfigLock.Unlock()

	logger.Debugf("[+] Deleting api id: %s", apiId)
	if _, present := pluginConfig[apiId]; present {
		err := deleteApiUterances(apiId, pluginConfig[apiId])
		if err != nil {
			logger.Errorf("[+] Error while deleting utterances for api id %s: %s", apiId, err)
			return
		}
		delete(pluginConfig, apiId)
	}

	apiSpecIndicesLock.Lock()
	delete(apiSpecIndices, apiId)
	apiSpecIndicesLock.Unlock()
}

func updatePluginConfig(apiId string, r *http.Request) error {
	pluginConfigLock.Lock()
	defer pluginConfigLock.Unlock()

	logger.Debugf("[+] Updating api id: %s", apiId)
	apiDef := getOASDefinition(r)
	// TOOD: fallback on classic...
	if apiDef == nil {
		err := fmt.Errorf("API definition is nil")
		logger.Errorf("[+] updatePluginConfig: %s", err)
		return err
	}

	pluginDataConfig, err := initPluginFromRequest(apiId, apiDef)
	if err != nil {
		logger.Fatalf("[+] Unable to parse configuration data: %s", err)
		return err
	}

	pluginConfigLock.Lock()
	pluginConfig[apiId] = pluginDataConfig
	pluginConfigLock.Unlock()

	return nil
}
