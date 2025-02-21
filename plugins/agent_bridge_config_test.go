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
		configData     interface{}
		expectedResult string
	}{
		{
			map[string]interface{}{
				"azureConfig": map[string]string{
					"openAIEndpoint": "https://tests-agents.openai.azure.com",
					"openAIKey":      "xxx",
				},
			},
			"{\"azureConfig\":{\"openAIKey\":\"xxx\",\"openAIEndpoint\":\"https://tests-agents.openai.azure.com\",\"modelDeployment\":\"gpt-4o-mini\"},\"selectOperations\":{},\"selectModelEmbedding\":\"MiniLM-L6-v2.Q8_0.gguf\",\"selectModelsPath\":\"models\",\"llmConfig\":null,\"APIID\":\"httpbin\"}",
		},
		{
			map[string]interface{}{},
			"{\"azureConfig\":{\"openAIKey\":\"\",\"openAIEndpoint\":\"https://api.openai.com/v1\",\"modelDeployment\":\"gpt-4o-mini\"},\"selectOperations\":{},\"selectModelEmbedding\":\"MiniLM-L6-v2.Q8_0.gguf\",\"selectModelsPath\":\"models\",\"llmConfig\":null,\"APIID\":\"httpbin\"}",
		},
	}

	for _, test := range tests {
		fmt.Printf("-------------------------------------------\n")
		// convert config to json
		configJSON, err := json.Marshal(test.configData)
		assert.Nil(t, err)

		// convert config json to map
		configMap := make(map[string]interface{})
		err = json.Unmarshal(configJSON, &configMap)
		assert.Nil(t, err)

		dataconfig, err := parseConfigData("httpbin", configMap)
		assert.Nil(t, err)
		assert.NotNil(t, dataconfig)

		jsonData, err := json.Marshal(dataconfig)
		assert.Nil(t, err)
		assert.Equal(t, test.expectedResult, string(jsonData), "Expected: %s, Got: %s", test.expectedResult, string(jsonData))
	}
}
