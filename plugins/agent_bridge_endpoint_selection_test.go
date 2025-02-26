// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kelindar/search"
	"github.com/stretchr/testify/assert"
)

const APIID_TO_TEST = "tyk-github-id"

func initForTests() error {
	pluginConfigForTest := []byte(`
{
  "tyk-github-id": {
    "azureConfig": {
      "openAIKey": "xxx",
      "openAIEndpoint": "https://smith-project-agents.openai.azure.com",
      "modelDeployment": "gpt-4o-mini"
    },
    "selectOperations": {
      "gists/create-comment": {
        "x-nl-input-examples": [
          "Add a comment to my gist titled 'Python Utilities' with my feedback: 'The second line must be fixed'",
          "Create a comment on the gist with ID 20384fe099243335bc81c608ea89e1de: 'That is wonderfull'"
        ]
      },
      "gists/list-comments": {
        "x-nl-input-examples": [
          "List comments of GIST 20384fe099243335bc81c608ea89e1de",
          "What are the feedback I've received on my gist 20384fe099243335bc81c608ea89e1de ?",
          "Is there negative comment on my most recent gist ?"
        ]
      },
      "gists/list-public": {
        "x-nl-input-examples": [
          "Fetch the latest public gists and filter them by creation date",
          "I only want the first 10 entries of the 4th page. Only give me the descriptions in markdown, and translate them in french",
          "I'm looking to display only the file names of public gists from the second page",
          "Can you download the content of all public gists that include Python files ?",
          "I intend to read and review the titles of all public gists created this week",
          "I aim to access public gists and read the tags applied to each one"
        ]
      },
      "gists/list-starred": {
        "x-nl-input-examples": [
          "I want to see which gists I've starred recently and read their descriptions",
          "What are the gists that I prefer ?",
          "I need to access my starred gists on GitHub to share a specific one with a colleague",
          "Give me my starred gists about zig programming language"
        ]
      },
      "issues/add-labels": {
        "x-nl-input-examples": [
          "Add label 'bug' to issue 6821 of "
        ]
      },
      "issues/create": {
        "x-nl-input-examples": [
          "Create a bug in repo",
          "I want to say that there is a vulnerability issue",
          "Add an issue about",
          "Create an issue about"
        ]
      },
      "issues/list-comments": {
        "x-nl-input-examples": [
          "List comments for issue",
          "List comments for issue with 50 results per page"
        ]
      },
      "issues/list-for-repo": {
        "x-nl-input-examples": [
          "List the issues for the repository",
          "Give issues in the repository to address any pending items",
          "Issues that are already closed",
          "Give me the last issues on repo for",
          "Give me last issues on repo.",
          "Give me issues assigned to",
          "Give me issues list",
          "last 3 issues in the repo"
        ]
      },
      "pulls/create": {
        "x-nl-input-examples": [
          "Create a pull request for repository",
          "From branch, create a pull request to "
        ]
      },
      "pulls/list": {
        "x-nl-input-examples": [
          "Give me the list of pull requests for repository",
          "show me pull requests for repository"
        ]
      },
      "repos/get-readme": {
        "x-nl-input-examples": [
          "Show me the README for repository owned by",
          "Give readme.md content in the repository"
        ]
      },
      "repos/list-commits": {
        "x-nl-input-examples": [
          "List the commits for repository",
          "Give commits in",
          "Commits created by author",
          "last 3 commits in",
          "last 3 commits in the repo created by author.",
          "Give commits in the repo created by author on branch",
          "last 3 commits in the repo",
          "Give me the last commits on repo for"
        ]
      },
      "repos/list-releases": {
        "x-nl-input-examples": [
          "List the releases for repository owned by",
          "Give releases in repository"
        ]
      },
      "repos/list-tags": {
        "x-nl-input-examples": [
          "List the tags for repository",
          "Give tags in the repository by"
        ]
      }
    },
    "selectModelEmbedding": "MiniLM-L6-v2.Q8_0.gguf",
    "selectModelsPath": "../tyk-release-v5.8.0-alpha8/models",
    "llmConfig": {
      "AzureConfig": {
        "openAIKey": "xxx",
        "openAIEndpoint": "https://smith-project-agents.openai.azure.com",
        "modelDeployment": "gpt-4o-mini"
      }
    },
    "APIID": "tyk-github-id"
  }
}
`)
	OPENAPI_KEY := os.Getenv("OPENAI_API_KEY")

	if err := json.Unmarshal(pluginConfigForTest, &pluginConfig); err != nil {
		return fmt.Errorf("conversion error for pluginConfig: %s", err)
	}
	pluginDataConfig := pluginConfig[APIID_TO_TEST]
	pluginDataConfig.AzureConfig.OpenAIKey = OPENAPI_KEY
	pluginDataConfig.LlmConfig.AzureConfig.OpenAIKey = OPENAPI_KEY

	if pluginDataConfig.AzureConfig.OpenAIKey == "" {
		return fmt.Errorf("Missing required config for azureConfig.openAIKey")
	}

	modelPath := filepath.Join(pluginDataConfig.SelectModelsPath, pluginDataConfig.SelectModelEmbedding)
	modelEmbedder, err := search.NewVectorizer(modelPath, 1)
	if err != nil {
		return fmt.Errorf("Unable to find embedding model %s: %s", pluginDataConfig.SelectModelEmbedding, err)
	}
	embeddingModels[pluginDataConfig.SelectModelEmbedding] = modelEmbedder
	dump("embeddingModels: ", embeddingModels)

	if err := initSelectOperations(APIID_TO_TEST, pluginDataConfig); err != nil {
		return fmt.Errorf("can't init operations for testing: %s", err)
	}

	return nil
}

func TestEndpointSelection(t *testing.T) {
	err := initForTests()
	assert.Nil(t, err)

	var tests = []struct {
		query             string
		expectedOperation string
		reachThreshold    bool
	}{
		{
			"Give me the list of pull requests for repository",
			"pulls/list",
			true,
		},
		{
			"Give me the 5 last issues on repo tyk owned by TykTechnologies.",
			"issues/list-for-repo",
			true,
		},
		{
			"Give me the 5 last issues on repo apiclarity owned by thelasttoto",
			"issues/list-for-repo",
			true,
		},
		{
			"Give me the 5 last commits on repo tyk owned by TykTechnologies.",
			"repos/list-commits",
			true,
		},
		{
			"Give me the 5 last commits on repo apiclarity owned by thelasttoto",
			"repos/list-commits",
			true,
		},
		{
			"Create a bug ",
			"issues/create",
			true,
		},
		{
			"Create a bug in the repo 'thelasttoto/apiclarity'",
			"issues/create",
			true,
		},
		{
			"Create a bug in the repo 'thelasttoto/apiclarity' about apiclarity crashing when compiled for linux, and assign it to user thelasttoto",
			"issues/create",
			true,
		},
		{
			"Donnes moi les 5 derniers commits du repo apiclarity de thelasttoto",
			"repos/list-commits",
			true,
		},
		{
			"Donnes moi les 5 derniers problemes du repo apiclarity de thelasttoto",
			"issues/list-for-repo",
			true,
		},
	}

	for _, test := range tests {
		fmt.Printf("----------------------------------------------\ninput=%v\n", test.query)
		matchingOperation, matchingScore, err := findSelectOperation(APIID_TO_TEST, test.query)
		assert.Nil(t, err)
		fmt.Printf("... matchingOperation=%v, matchingScore=%v\n", *matchingOperation, matchingScore)
		assert.Equal(t, test.expectedOperation, *matchingOperation)
		assert.Equal(t, test.reachThreshold, (matchingScore >= RELEVANCE_THRESHOLD))
	}
}
