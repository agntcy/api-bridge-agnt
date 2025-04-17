// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kelindar/search"
	"github.com/stretchr/testify/assert"
)

type EndpointSelectionTestingRequests struct {
	TargetApiID       string `json:"api_id"`
	Query             string `json:"query"`
	ExpectedOperation string `json:"expected"`
	ReachThreshold    bool   `json:"reach_threshold"`
}

var SpecToTests = []struct {
	apiId        string
	specFilename string
}{
	{"tyk-gmail-id", "../configs/gmail.googleapis.com.oas.json"},
	{"tyk-jira-id", "../configs/your-domain.atlassian.net.oas.json"},
	{"tyk-github-id", "../configs/api.github.com.gist.deref.oas.json"},
	{"tyk-sendgrid-id", "../configs/api.sendgrid.com.oas.json"},
}

func loadApiSpecsForTests(apiId string, specFilename string) (*PluginDataConfig, error) {
	jsonFile, err := os.Open(specFilename)
	if err != nil {
		return nil, fmt.Errorf("can't open file %s: %s", specFilename, err)
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)

	pluginDataConfig := &PluginDataConfig{
		AzureConfig: AzureConfig{
			OpenAIEndpoint:  "xx", // Don't care for the semantic router tests
			OpenAIKey:       "xx", // Don't care for the semantic router tests
			ModelDeployment: DEFAULT_OPENAI_MODEL,
		},
		SelectOperations:     map[string]*AIExtensionConfig{},
		SelectModelEmbedding: DEFAULT_MODEL_EMBEDDINGS_MODEL,
		SelectModelsPath:     "../tyk-release-v5.8.0/models",

		APIID: apiId,
	}

	var result map[string]interface{}
	err = json.Unmarshal([]byte(byteValue), &result)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal file %s: %s", specFilename, err)
	}

	for _, pathData := range result["paths"].(map[string]interface{}) {
		for method, methodData := range pathData.(map[string]interface{}) {
			if slices.Contains([]string{"get", "post", "put", "patch", "delete"}, method) {
				haveInputExamples := false
				operationId := ""
				uterances := []string{}

				for item, itemData := range methodData.(map[string]interface{}) {
					if item == "operationId" {
						operationId = itemData.(string)
					}
					if item == SPEC_EXT_AI_INPUT_EXAMPLES {
						haveInputExamples = true
						for _, example := range itemData.([]interface{}) {
							uterances = append(uterances, example.(string))
						}
					}
				}

				if haveInputExamples {
					pluginDataConfig.SelectOperations[operationId] = &AIExtensionConfig{
						InputExamples: uterances,
					}
				}
			}
		}
	}

	return pluginDataConfig, nil
}

func initConfigFromApiSpecsForTests() error {
	for _, specData := range SpecToTests {
		pluginDataConfig, err := loadApiSpecsForTests(specData.apiId, specData.specFilename)
		if err != nil {
			return fmt.Errorf("can't load api specs for tests: %s", err)
		}
		pluginConfig[specData.apiId] = pluginDataConfig
	}

	return nil
}

func initForTests() error {
	for apiID, pluginDataConfig := range pluginConfig {
		modelPath := filepath.Join(pluginDataConfig.SelectModelsPath, DEFAULT_MODEL_EMBEDDINGS_MODEL)
		modelEmbedder, err := search.NewVectorizer(modelPath, 1)
		if err != nil {
			return fmt.Errorf("Unable to find embedding model %s: %s", pluginDataConfig.SelectModelEmbedding, err)
		}
		embeddingModels[pluginDataConfig.SelectModelEmbedding] = modelEmbedder

		if err := initSelectOperations(apiID, pluginDataConfig); err != nil {
			return fmt.Errorf("can't init operations for testing: %s", err)
		}
	}
	return nil
}

func loadRequestToTest(filename string) ([]EndpointSelectionTestingRequests, error) {
	var tests []EndpointSelectionTestingRequests
	file, err := os.Open(filename)
	if err != nil {
		return tests, fmt.Errorf("can't open file %s: %s", filename, err)
	}
	defer file.Close()
	byteValue, _ := io.ReadAll(file)
	err = json.Unmarshal(byteValue, &tests)
	if err != nil {
		return tests, fmt.Errorf("can't unmarshal file %s: %s", filename, err)
	}
	return tests, nil
}

func TestEndpointSelection(t *testing.T) {
	// Init the config directly from the API specs
	err := initConfigFromApiSpecsForTests()
	assert.Nil(t, err)

	// Init the models for each API
	err = initForTests()
	assert.Nil(t, err)

	// Load the query to tests from JSON file
	tests, err := loadRequestToTest("./testdata/endpoint_selection_requests_to_test.json")
	assert.Nil(t, err)

	for _, tt := range tests {
		t.Run(tt.Query, func(t *testing.T) {
			if len(tt.ExpectedOperation) == 0 {
				t.Skip("No expected operation for query: " + tt.Query)
			}
			_, ok := pluginConfig[tt.TargetApiID]
			if ok {
				matchingOperation, matchingScore, err := findSelectOperation(tt.TargetApiID, tt.Query)

				assert.Nil(t, err)
				assert.Equal(t, tt.ExpectedOperation, *matchingOperation)
				assert.Equal(t, tt.ReachThreshold, (matchingScore >= DEFAULT_RELEVANCE_THRESHOLD))
			} else {
				t.Skip("No plugin config found for api: " + tt.TargetApiID)
			}
		})
	}

}
