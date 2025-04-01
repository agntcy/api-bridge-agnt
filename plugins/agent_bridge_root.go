package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/TykTechnologies/tyk/storage"

	"github.com/kelindar/search"
)

const (
	DEFAULT_THRESHOLD = 0.5
)

type ACPPluginApiConfig struct {
	APIName    string   `json:"name"`
	Target     string   `json:"url"`
	Utterances []string `json:"utterances"`
}

type ACPPluginData struct {
	ACPPluginServices map[string]ACPPluginApiConfig
	ModelPath         string
	ModelEmbedder     *search.Vectorizer
	ModelIndex        *search.Index[string]
	Store             *storage.RedisCluster
}

var acpPluginData = ACPPluginData{
	ACPPluginServices: map[string]ACPPluginApiConfig{},
}

func initACPPlugin(ctx context.Context) {
	if acpPluginData.Store == nil {
		acpPluginData.Store = getStorageForPlugin(ctx)
	}
}

func initACPPluginApiConfig(r *http.Request) error {
	initACPPlugin(r.Context())

	apiKeysValues := acpPluginData.Store.GetKeysAndValuesWithFilter("*")
	if apiKeysValues == nil {
		logger.Error("[+] Error while getting the keys and values from Redis")
		return fmt.Errorf("error while getting the keys and values from Redis")
	}
	logger.Debug("[+] ----------------- READING KEYS AND VALUES FROM REDIS -----------------")
	for key, value := range apiKeysValues {
		logger.Debugf("[+] Key: %s, Value: %s", key, value)
		apiConfig := ACPPluginApiConfig{}
		err := json.Unmarshal([]byte(value), &apiConfig)
		if err != nil {
			return fmt.Errorf("conversion error for acpPluginConfig: %s", err)
		}
		acpPluginData.ACPPluginServices[apiConfig.APIName] = apiConfig
	}
	logger.Debug("[+] ---------------------------------------------------------------------")

	return nil
}

func findACPServiceFromQuery(query string) (string, error) {
	logger.Debug("[+] ###AXT:: findACPServiceFromQuery -->")
	logger.Infof("[+] ###AXT:: query=%v <--", query)

	if acpPluginData.ModelEmbedder == nil {
		var err error
		acpPluginData.ModelPath = filepath.Join(DEFAULT_MODEL_EMBEDDINGS_PATH, DEFAULT_MODEL_EMBEDDINGS_MODEL)
		acpPluginData.ModelEmbedder, err = search.NewVectorizer(acpPluginData.ModelPath, 1)
		if err != nil {
			return "", fmt.Errorf("[+] Unable to find embedding model %s: %s", acpPluginData.ModelPath, err)
		}
		acpPluginData.ModelIndex = search.NewIndex[string]()
		for _, service := range acpPluginData.ACPPluginServices {
			for _, utterance := range service.Utterances {
				embedding, err := acpPluginData.ModelEmbedder.EmbedText(utterance)
				if err != nil {
					return "", fmt.Errorf("[+] embedding model %s failed for text \"%s\": %s", acpPluginData.ModelPath, utterance, err)
				}
				acpPluginData.ModelIndex.Add(embedding, service.Target)
			}
		}
	}
	if acpPluginData.ModelEmbedder == nil || acpPluginData.ModelIndex == nil {
		return "", fmt.Errorf("[+] ModelEmbedder or ModelIndex is nil")
	}

	embedding, err := acpPluginData.ModelEmbedder.EmbedText(query)
	if err != nil {
		return "", fmt.Errorf("[+] embedding model %s failed for query \"%s\": %s", acpPluginData.ModelPath, query, err)
	}
	results := acpPluginData.ModelIndex.Search(embedding, 5)
	for _, r := range results {
		logger.Debugf("Result: %s (Relevance: %.2f)\n", r.Value, r.Relevance)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("[+] No service found for query \"%s\": %s", query, err)
	}
	if results[0].Relevance < DEFAULT_THRESHOLD {
		return "", fmt.Errorf("[+] No valid service found for query \"%s\": %s", query, err)
	}
	logger.Infof("[+] service found (%v) for query (%v) <--", results[0].Value, query)

	logger.Debug("[+] ###AXT:: findACPServiceFromQuery <--")
	return results[0].Value, nil
}
