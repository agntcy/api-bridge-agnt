// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import "fmt"

const NBRESULT = 1

type QueryEndpointSelectionMatch struct {
	Query       string  `json:"query"`
	OperationId string  `json:"operationId,omitempty"`
	Relevance   float64 `json:"relevance,omitempty"`
	Error       string  `json:"error,omitempty"`
}

type QueryEndpointSelectionReply struct {
	Results []QueryEndpointSelectionMatch `json:"results"`
}

func findSelectOperation(apiId string, input string) (*string, float64, error) {
	apiSpecIndicesLock.RLock()
	apiSpecIndex, present := apiSpecIndices[apiId]
	apiSpecIndicesLock.RUnlock()
	if !present {
		// This API has no x-nl-input-examples
		return nil, 0, fmt.Errorf("no x-nl-input-examples found for api id: %s", apiId)
	}
	pluginConfigLock.RLock()
	pluginDataConfig, present := pluginConfig[apiId]
	pluginConfigLock.RUnlock()
	if !present {
		return nil, 0, fmt.Errorf("no plugin config found for api: %s", apiId)
	}
	embeddingModelsLock.RLock()
	modelEmbedder, present := embeddingModels[pluginDataConfig.SelectModelEmbedding]
	embeddingModelsLock.RUnlock()
	if !present {
		return nil, 0, fmt.Errorf("no embedding model found for api id: %s", apiId)
	}
	embedding, err := modelEmbedder.EmbedText(input)
	if err != nil {
		return nil, 0, err
	}
	results := apiSpecIndex.Search(embedding, NBRESULT)
	if len(results) < 1 {
		return nil, 0, nil
	} else {
		if NBRESULT > 1 {
			for index, result := range results {
				logger.Debugf("Result %d: %v / %v\n", index, result.Value, result.Relevance)
			}
		}
		return &results[0].Value, results[0].Relevance, nil
	}
}

func selectEndpoint(apiID string, selectionQueries []string) []QueryEndpointSelectionMatch {
	matches := make([]QueryEndpointSelectionMatch, len(selectionQueries))
	for _, selectionQuery := range selectionQueries {
		if len(selectionQuery) == 0 {
			continue
		}

		matchingOperation, matchingScore, err := findSelectOperation(apiID, selectionQuery)

		match := QueryEndpointSelectionMatch{
			Query:     selectionQuery,
			Relevance: matchingScore,
		}
		if matchingOperation != nil {
			match.OperationId = *matchingOperation
		}
		if err != nil {
			match.Error = err.Error()
		} else {
			match.Error = "ok"
		}
		matches = append(matches, match)
	}
	return matches
}
