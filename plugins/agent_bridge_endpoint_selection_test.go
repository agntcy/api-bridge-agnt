// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kelindar/search"
	"github.com/stretchr/testify/assert"
)

const APIID_TO_TEST = "tyk-github-id"

func initForTests() error {

	jsonFile, err := os.Open("./testdata/config_for_testing.json")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened users.json")
	defer jsonFile.Close()
	pluginConfigForTest, _ := io.ReadAll(jsonFile)

	if err := json.Unmarshal(pluginConfigForTest, &pluginConfig); err != nil {
		return fmt.Errorf("conversion error for pluginConfig: %s", err)
	}

	for apiID, pluginDataConfig := range pluginConfig {
		pluginDataConfig.SelectModelEmbedding = DEFAULT_MODEL_EMBEDDINGS_MODEL

		if pluginDataConfig.AzureConfig.OpenAIKey == "" {
			return fmt.Errorf("Missing required config for azureConfig.openAIKey")
		}

		modelPath := filepath.Join(pluginDataConfig.SelectModelsPath, pluginDataConfig.SelectModelEmbedding)
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

func TestEndpointSelection(t *testing.T) {
	err := initForTests()
	assert.Nil(t, err)

	tests := []struct {
		targetApiID       string
		query             string
		expectedOperation string
		reachThreshold    bool
	}{
		{
			"tyk-github-id",
			"Give me the list of pull requests for repository",
			"pulls/list",
			true,
		},
		{
			"tyk-github-id",
			"Give me the 5 last issues on repo tyk owned by TykTechnologies.",
			"issues/list-for-repo",
			true,
		},
		{
			"tyk-github-id",
			"Give me the 5 last issues on repo apiclarity owned by thelasttoto",
			"issues/list-for-repo",
			true,
		},
		{
			"tyk-github-id",
			"Give me the 5 last commits on repo tyk owned by TykTechnologies.",
			"repos/list-commits",
			true,
		},
		{
			"tyk-github-id",
			"Give me the 5 last commits on repo apiclarity owned by thelasttoto",
			"repos/list-commits",
			true,
		},
		{
			"tyk-github-id",
			"Create a bug ",
			"issues/create",
			true,
		},
		{
			"tyk-github-id",
			"Create a bug in the repo 'thelasttoto/apiclarity'",
			"issues/create",
			true,
		},
		{
			"tyk-github-id",
			"Create a bug in the repo 'thelasttoto/apiclarity' about apiclarity crashing when compiled for linux, and assign it to user thelasttoto",
			"issues/create",
			true,
		},
		{
			"tyk-github-id",
			"Donnes moi les 5 derniers commits du repo apiclarity de thelasttoto",
			"repos/list-commits",
			true,
		},
		{
			"tyk-github-id",
			"Donnes moi les 5 derniers problemes du repo apiclarity de thelasttoto",
			"issues/list-for-repo",
			true,
		},
		{
			"tyk-jira-id",
			"what is the last issues of project PUCCINI",
			"getRecent",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			matchingOperation, matchingScore, err := findSelectOperation(tt.targetApiID, tt.query)

			assert.Nil(t, err)
			assert.Equal(t, tt.expectedOperation, *matchingOperation)
			assert.Equal(t, tt.reachThreshold, (matchingScore >= RELEVANCE_THRESHOLD))
		})
	}
}
