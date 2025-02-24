// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigParseConfigData(t *testing.T) {
	var tests = []struct {
		configData     any
		expectedConfig PluginDataConfig
	}{
		{
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
				SelectModelEmbedding: "MiniLM-L6-v2.Q8_0.gguf",
				SelectModelsPath:     "models",
				APIID:                "httpbin",
			},
		},
		{
			map[string]any{},
			PluginDataConfig{
				AzureConfig: AzureConfig{
					OpenAIEndpoint:  "https://api.openai.com/v1",
					OpenAIKey:       "",
					ModelDeployment: "gpt-4o-mini",
				},
				SelectOperations:     map[string]*AIExtensionConfig{},
				SelectModelEmbedding: "MiniLM-L6-v2.Q8_0.gguf",
				SelectModelsPath:     "models",
				APIID:                "httpbin",
			},
		},
	}

	for _, test := range tests {
		fmt.Printf("-------------------------------------------\n")
		// convert config to json
		configJSON, err := json.Marshal(test.configData)
		assert.Nil(t, err)

		// convert config json to map
		configMap := make(map[string]any)
		err = json.Unmarshal(configJSON, &configMap)
		assert.Nil(t, err)

		dataconfig, err := parseConfigData("httpbin", configMap)
		assert.Nil(t, err)
		assert.NotNil(t, dataconfig)

		expectedDataConfig, err := json.Marshal(test.expectedConfig)
		assert.Nil(t, err)

		jsonData, err := json.Marshal(dataconfig)
		assert.Nil(t, err)
		assert.Equal(t, string(expectedDataConfig), string(jsonData), "Expected: %s, Got: %s", string(expectedDataConfig), string(jsonData))
	}
}
