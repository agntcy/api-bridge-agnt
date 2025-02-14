// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/TykTechnologies/tyk/ctx"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/kelindar/search"
	"github.com/spf13/viper"
)

const (
	SPEC_EXT_AI_INPUT_EXAMPLES = "x-nl-input-examples"

	DEFAULT_MODEL_EMBEDDINGS_PATH  = "models"
	DEFAULT_MODEL_EMBEDDINGS_MODEL = "MiniLM-L6-v2.Q8_0.gguf" // provided by search package dist
	DEFAULT_OPENAI_ENDPOINT        = "https://api.openai.com/v1"
	DEFAULT_OPENAI_MODEL           = "gpt-4o-mini"
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

	APIID string
}

func getApiId(r *http.Request) (string, error) {
	apidef := ctx.GetOASDefinition(r)
	if apidef == nil {
		// TOOD: fallback on classic...
		return "", fmt.Errorf("API definition is nil")
	}
	gateway := apidef.GetTykExtension()
	if gateway == nil {
		return "", fmt.Errorf("Tyk gateway definition is nil")
	}
	return gateway.Info.ID, nil
}

func parseConfigData(apiId string, configData map[string]interface{}) (*PluginDataConfig, error) {
	logger.Debugf("[+] Parsing config for api id: %s", apiId)
	defaultAzureConfig := map[string]string{"openAIEndpoint": DEFAULT_OPENAI_ENDPOINT, "modelDeployment": DEFAULT_OPENAI_MODEL}

	confViper := viper.New()
	confViper.SetDefault("selectOperations", map[string]*AIExtensionConfig{})
	confViper.SetDefault("selectModelEmbedding", DEFAULT_MODEL_EMBEDDINGS_MODEL)
	confViper.SetDefault("selectModelsPath", DEFAULT_MODEL_EMBEDDINGS_PATH)
	confViper.SetDefault("azureConfig", defaultAzureConfig)

	if len(configData) > 0 {
		if err := confViper.MergeConfigMap(configData); err != nil {
			return nil, err
		}
	}

	azureConfig := confViper.Sub("azureConfig")
	if azureConfig == nil {
		err := fmt.Errorf("failed to get subconfig AzureConfig")
		logger.Errorf("[+] Error reading configuration for AzureConfig: %s", err)
		return nil, err
	}
	// Propagate defaults. Viper doesn't seem to do this. :(
	for key, value := range defaultAzureConfig {
		azureConfig.SetDefault(key, value)
	}
	err1 := azureConfig.BindEnv("openAIEndpoint", "OPENAI_ENDPOINT")
	err2 := azureConfig.BindEnv("openAIKey", "OPENAI_API_KEY")
	err3 := azureConfig.BindEnv("modelDeployment", "OPENAI_MODEL")
	if err1 != nil || err2 != nil || err3 != nil {
		err := fmt.Errorf("failed to get subconfig AzureConfig")
		logger.Errorf("[+] Error reading configuration for AzureConfig: %s", err)
		return nil, err
	}

	selectOperations := map[string]*AIExtensionConfig{}
	for apiId, aiExtension := range confViper.GetStringMap("selectOperations") {
		aiExtViper := viper.New()

		if err := aiExtViper.MergeConfigMap(aiExtension.(map[string]interface{})); err != nil {
			logger.Errorf("[+] Error reading configuration for selectOperations: %s", err)
		} else {
			selectOperations[apiId] = &AIExtensionConfig{
				InputExamples: aiExtViper.GetStringSlice(SPEC_EXT_AI_INPUT_EXAMPLES),
			}
		}
	}

	pluginDataConfig := &PluginDataConfig{
		AzureConfig: AzureConfig{
			OpenAIEndpoint:  azureConfig.GetString("openAIEndpoint"),
			OpenAIKey:       azureConfig.GetString("openAIKey"),
			ModelDeployment: azureConfig.GetString("modelDeployment"),
		},
		SelectOperations:     selectOperations,
		SelectModelEmbedding: confViper.GetString("selectModelEmbedding"),
		SelectModelsPath:     confViper.GetString("selectModelsPath"),

		APIID: apiId,
	}

	logger.Debugf("[+] Finished parsing config for api id: %s", apiId)
	return pluginDataConfig, nil
}

func initPluginFromRequest(r *http.Request) (*PluginDataConfig, error) {
	apiID, err := getApiId(r)
	if err != nil {
		logger.Errorf("[+] initPluginFromRequest cannot find api id: %s", err)
		return nil, err
	}

	// Note: we really need to just to be able to clear the cache on API def
	// reloads to fix everything complicated.
	pluginConfigLock.RLock()
	pluginDataConfig, present := pluginConfig[apiID]
	pluginConfigLock.RUnlock()
	if present {
		logger.Debugf("[+] Config data already cached for api id: %s", apiID)
		return pluginDataConfig, nil
	}

	logger.Debugf("[+] Initializing for api id: %s", apiID)

	apidef := ctx.GetOASDefinition(r)
	// TOOD: fallback on classic...
	if apidef == nil {
		err := fmt.Errorf("API definition is nil")
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return nil, err
	}

	middleware := apidef.GetTykMiddleware()
	if middleware == nil {
		err := fmt.Errorf("Tyk middleware definition is nil")
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return nil, err
	}
	globalPluginConfig := middleware.Global.PluginConfig
	if globalPluginConfig == nil {
		err := fmt.Errorf("Tyk global.pluginConfig definition is nil")
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return nil, err
	}
	pluginConfigData := globalPluginConfig.Data
	if pluginConfigData == nil {
		err := fmt.Errorf("Tyk pluginConfig.data definition is nil")
		logger.Errorf("[+] initPluginFromRequest: %s", err)
		return nil, err
	}
	pluginDataConfig, err = parseConfigData(apiID, pluginConfigData.Value)
	if err != nil {
		logger.Fatalf("[+] Unable to parse configuration data: %s", err)
		return pluginDataConfig, err
	}

	// Iterate through all paths and operations in the API definition
	for _, path := range apidef.Paths {
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
			for _, example := range aiExamples.([]interface{}) {
				exampleStr, isString := example.(string)
				if !isString {
					logger.Fatalf("[+] Error parsing examples for operation %s", operationId)
				}
				aiExtentionConfig.InputExamples = append(aiExtentionConfig.InputExamples, exampleStr)
			}

			pluginDataConfig.SelectOperations[operationId] = aiExtentionConfig
		}
	}

	if pluginDataConfig.AzureConfig.OpenAIKey == "" {
		err := fmt.Errorf("Missing required config for azureConfig.openAIKey")
		logger.Fatalf("[+] Error initializing plugin: %s", err)
		return pluginDataConfig, err
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

	pluginConfigLock.Lock()
	pluginConfig[apiID] = pluginDataConfig
	pluginConfigLock.Unlock()

	if len(pluginDataConfig.SelectOperations) > 0 {
		// Note: create embedder before initializing indices!
		logger.Debugf("[+] Loading embedding model %s for api id: %s", pluginDataConfig.SelectModelEmbedding, apiID)
		embeddingModelsLock.RLock()
		_, present := embeddingModels[pluginDataConfig.SelectModelEmbedding]
		embeddingModelsLock.RUnlock()
		if present {
			logger.Debugf("[+] embedding model %s cached for api id: %s", pluginDataConfig.SelectModelEmbedding, apiID)
		} else {
			modelPath := filepath.Join(pluginDataConfig.SelectModelsPath, pluginDataConfig.SelectModelEmbedding)
			embeddingModelsLock.Lock()
			_, present := embeddingModels[pluginDataConfig.SelectModelEmbedding]
			if !present {
				modelEmbedder, err := search.NewVectorizer(modelPath, 1)
				if err != nil {
					embeddingModelsLock.Unlock()
					logger.Fatalf("[+] Unable to find embedding model %s: %s", pluginDataConfig.SelectModelEmbedding, err)
					return pluginDataConfig, nil
				}
				embeddingModels[pluginDataConfig.SelectModelEmbedding] = modelEmbedder
			}
			embeddingModelsLock.Unlock()
			if !present {
				logger.Debugf("[+] Added embedding model %s for api id: %s", pluginDataConfig.SelectModelEmbedding, apiID)
			}
		}

		if err := initSelectOperations(apiID, pluginDataConfig); err != nil {
			logger.Fatalf("[+] failed to initialize select operations for api id %s: %s", apiID, err)
			return pluginDataConfig, err
		}
	}

	logConfig()

	logger.Debugf("[+] Finished initPluginFromRequest fror api id: %s", apiID)
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
	for apiId, pluginDataConfig := range pluginConfig {
		logger.Infof("[+] Config %s: Azure OpenAI API Key: %s", apiId, "**REDACTED**")
		logger.Infof("[+] Config %s: Azure OpenAI Endpoint: %s", apiId, pluginDataConfig.AzureConfig.OpenAIEndpoint)
		logger.Infof("[+] Config %s: Azure OpenAI Model Deployment ID: %s", apiId, pluginDataConfig.AzureConfig.ModelDeployment)
		if len(pluginDataConfig.SelectOperations) > 0 {
			logger.Infof("[+] Config %s: Select operations: %d", apiId, len(pluginDataConfig.SelectOperations))
			logger.Infof("[+] Config %s: Select embedding model: %s", apiId, filepath.Join(pluginDataConfig.SelectModelsPath, pluginDataConfig.SelectModelEmbedding))
		}
	}
	pluginConfigLock.RUnlock()
}
