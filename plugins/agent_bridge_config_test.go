// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigParseConfigData(t *testing.T) {
	tests := []struct {
		description    string
		configData     any
		expectedConfig PluginDataConfig
	}{
		{
			"Some default values",
			map[string]any{
				"azureConfig": map[string]string{
					"openAIEndpoint": "https://tests-agents.openai.azure.com",
					"openAIKey":      "xxx",
				},
			},
			PluginDataConfig{
				AzureConfig: AzureConfig{
					OpenAIEndpoint:  "https://tests-agents.openai.azure.com",
					OpenAIKey:       "xxx",
					ModelDeployment: "gpt-4o-mini",
				},
				SelectOperations:     map[string]*AIExtensionConfig{},
				SelectModelEmbedding: DEFAULT_MODEL_EMBEDDINGS_MODEL,
				SelectModelsPath:     "models",
				APIID:                "httpbin",
			},
		},
		{
			"All default values",
			map[string]any{},
			PluginDataConfig{
				AzureConfig: AzureConfig{
					OpenAIEndpoint:  "https://api.openai.com/v1",
					OpenAIKey:       "",
					ModelDeployment: "gpt-4o-mini",
				},
				SelectOperations:     map[string]*AIExtensionConfig{},
				SelectModelEmbedding: DEFAULT_MODEL_EMBEDDINGS_MODEL,
				SelectModelsPath:     "models",
				APIID:                "httpbin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			// convert config to json
			configJSON, err := json.Marshal(tt.configData)
			assert.Nil(t, err)

			// convert config json to map
			configMap := make(map[string]any)
			err = json.Unmarshal(configJSON, &configMap)
			assert.Nil(t, err)

			dataconfig, err := parseConfigData("httpbin", configMap)
			assert.Nil(t, err)
			assert.NotNil(t, dataconfig)

			expectedDataConfig, err := json.Marshal(tt.expectedConfig)
			assert.Nil(t, err)

			jsonData, err := json.Marshal(dataconfig)
			assert.Nil(t, err)
			assert.Equal(t, string(expectedDataConfig), string(jsonData))
		})
	}
}
