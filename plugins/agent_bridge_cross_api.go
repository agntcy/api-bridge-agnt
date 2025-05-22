// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/TykTechnologies/tyk/ctx"
	"github.com/TykTechnologies/tyk/user"
	"github.com/kelindar/search"
)

const (
	DEFAULT_THRESHOLD = 0.5
)

type apiServicePluginApiConfig struct {
	APIName    string   `json:"name"`
	Target     string   `json:"url"`
	Utterances []string `json:"utterances"`
}

type apiServicePluginData struct {
	PluginServices map[string]apiServicePluginApiConfig
	ModelPath         string
	ModelEmbedder     *search.Vectorizer
	ModelIndex        *search.Index[string]
	StoreVersion      int64
	MaxRequestLength  int64 `json:"maxRequestLength"` // MaxRequestSize is the maximum size of the request in characters; default is -1 (no limit)
}

var servicePluginData = apiServicePluginData{
	PluginServices: map[string]apiServicePluginApiConfig{},
}

// SetContext updates the context of a request.
func SetContext(r *http.Request, ctx context.Context) {
	r2 := r.WithContext(ctx)
	*r = *r2
}

func processSelectAPI(rw http.ResponseWriter, r *http.Request) {
	logger.Debug("[+] Inside processSelectAPI -->")

	if len(servicePluginData.PluginServices) == 0 || servicePluginData.StoreVersion != storeVersion {
		logger.Infof("[+] config is empty or store version has changed, reloading ...")
		err := initServicePluginApiConfig()
		if err != nil {
			logger.Errorf("[+] Error while getting the plugin config: %s", err)
			http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
			return
		}
	}

	if servicePluginData.MaxRequestLength > 0 && r.ContentLength > servicePluginData.MaxRequestLength {
		logger.Debugf("[+] Query is too large, ignoring ...")
		http.Error(rw, "Query is too large", http.StatusRequestEntityTooLarge)
		return
	}

	nlqBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("[+] Error while reading the body: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}
	nlq := string(nlqBytes)

	session := &user.SessionState{
		MetaData: map[string]any{
			METADATA_NLQ:           string(nlq),
			METADATA_RESPONSE_TYPE: RESPONSE_TYPE_NL,
		},
	}
	ctx.SetSession(r, session, true)

	service, err := findServiceFromQuery(nlq)
	if err != nil {
		logger.Errorf("[+] Failed to find a service for query: %s", nlq)
		http.Error(rw, NO_SERVICE_FOUND, http.StatusNotFound)
		return
	}
	logger.Debugf("[+] Found a service (%v) for query=%v", service, nlq)

	u, err := url.Parse(service)
	if err != nil {
		logger.Errorf("[+] Error while parsing the service URL (%v): %s", service, err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}
	logger.Debugf("[+] redirect query to: %v ", u)

	rctx := r.Context()
	rctx = context.WithValue(rctx, ctx.UrlRewriteTarget, u)
	SetContext(r, rctx)
}

func initServicePluginApiConfig() error {
	// Clear existing map
	servicePluginData.PluginServices = make(map[string]apiServicePluginApiConfig)

	// save the current version of the store BEFORE retreiving the data
	servicePluginData.StoreVersion = storeVersion
	logger.Debugf("[+] Loading plugin config version (%v) ...", servicePluginData.StoreVersion)

	servicePluginData.MaxRequestLength = int64(getEnvAsInt("MAX_REQUEST_SIZE", DEFAULT_MAX_REQUEST_SIZE))

	// Get All APIs keys and values from Redis
	apiKeysValues := agentBridgeStore.GetKeysAndValuesWithFilter("*")
	if apiKeysValues == nil {
		logger.Error("[+] Error while getting the keys and values from Redis")
		return fmt.Errorf("error while getting the keys and values from Redis")
	}
	// Refresh config
	for key, value := range apiKeysValues {
		logger.Debugf("[+] Found key: '%s', with value: '%s'", key, value)
		apiConfig := apiServicePluginApiConfig{}
		err := json.Unmarshal([]byte(value), &apiConfig)
		if err != nil {
			logger.Fatalf("[+] conversion error for apiServicePluginApiConfig: %s", err)
		}
		servicePluginData.PluginServices[apiConfig.APIName] = apiConfig
	}

	return nil
}

func findServiceFromQuery(query string) (string, error) {
	logger.Debugf("[+] Process query=%v <--", query)

	if servicePluginData.ModelEmbedder == nil {
		var err error
		servicePluginData.ModelPath = filepath.Join(DEFAULT_MODEL_EMBEDDINGS_PATH, DEFAULT_MODEL_EMBEDDINGS_MODEL)
		servicePluginData.ModelEmbedder, err = search.NewVectorizer(servicePluginData.ModelPath, 1)
		if err != nil {
			return "", fmt.Errorf("[+] Unable to find embedding model %s: %s", servicePluginData.ModelPath, err)
		}
		servicePluginData.ModelIndex = search.NewIndex[string]()
		for _, service := range servicePluginData.PluginServices {
			for _, utterance := range service.Utterances {
				embedding, err := servicePluginData.ModelEmbedder.EmbedText(utterance)
				if err != nil {
					return "", fmt.Errorf("[+] embedding model %s failed for text \"%s\": %s", servicePluginData.ModelPath, utterance, err)
				}
				servicePluginData.ModelIndex.Add(embedding, service.Target)
			}
		}
	}
	if servicePluginData.ModelEmbedder == nil || servicePluginData.ModelIndex == nil {
		return "", fmt.Errorf("[+] ModelEmbedder or ModelIndex is nil")
	}

	embedding, err := servicePluginData.ModelEmbedder.EmbedText(query)
	if err != nil {
		return "", fmt.Errorf("[+] embedding model %s failed for query \"%s\": %s", servicePluginData.ModelPath, query, err)
	}
	results := servicePluginData.ModelIndex.Search(embedding, NBRESULT)
	if len(results) == 0 {
		return "", fmt.Errorf("[+] No service found for query \"%s\": %s", query, err)
	} else if NBRESULT > 1 {
		for index, result := range results {
			logger.Debugf("Result %d: %v / %v\n", index, result.Value, result.Relevance)
		}
	}
	if results[0].Relevance < DEFAULT_THRESHOLD {
		return "", fmt.Errorf("[+] No valid service found for query \"%s\": %s", query, err)
	}

	return results[0].Value, nil
}

func init() {
	logger.Infof("[+] Initializing API Bridge Agent plugin ...")

	// Init Redis store, if needed
	if agentBridgeStore == nil {
		agentBridgeStore = getStorageForPlugin(context.TODO())
	}
}
