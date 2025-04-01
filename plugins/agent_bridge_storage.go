// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TykTechnologies/tyk/config"
	"github.com/TykTechnologies/tyk/storage"
)

const (
	AGENT_BRIDGE_DEFAULT_KEY_PREFIX = "agent_bridge:"
	AGENT_BRIDGE_DEFAULT_TTL        = 30240000
)

func getStorageForPlugin(ctx context.Context) *storage.RedisCluster {
	rc := storage.NewConnectionHandler(ctx)

	go rc.Connect(ctx, nil, &config.Config{})
	rc.WaitConnect(ctx)

	handler := &storage.RedisCluster{KeyPrefix: AGENT_BRIDGE_DEFAULT_KEY_PREFIX, ConnectionHandler: rc}
	handler.Connect()
	return handler
}

func saveApiUterances(apiID string, pluginDataConfig *PluginDataConfig) error {
	pluginConfigLock.Lock()
	defer pluginConfigLock.Unlock()

	if pluginDataConfig == nil {
		return fmt.Errorf("pluginDataConfig is nil")
	}
	if pluginDataConfig.Store == nil {
		return fmt.Errorf("pluginDataConfig storage is not configured")
	}

	utterances := []string{}
	for _, aiExtention := range pluginDataConfig.SelectOperations {
		utterances = append(utterances, aiExtention.InputExamples...)
	}
	apiConfig := ACPPluginApiConfig{
		APIName:    pluginDataConfig.APIID,
		Target:     fmt.Sprintf("tyk://%s%s", apiID, pluginDataConfig.ListenPath),
		Utterances: utterances,
	}
	/*err := json.Unmarshal([]byte(value), &apiConfig)
	if err != nil {
		return fmt.Errorf("conversion error for acpPluginConfig: %s", err)
	}*/

	jsonApiConfig, err := json.Marshal(apiConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal struct: %v, err=%v", utterances, err)
	}

	logger.Debugf("[+] ###AXT:: Save config to redis key : %s", apiID)
	// Save the plugin data config to the Redis store
	if err := pluginDataConfig.Store.SetKey(apiID, string(jsonApiConfig), AGENT_BRIDGE_DEFAULT_TTL); err != nil {
		return fmt.Errorf("failed to save plugin data config: %w", err)
	}

	return nil
}

func loadApiUterances(apiID string, pluginDataConfig *PluginDataConfig) ([]string, error) {
	utterances := []string{}

	if pluginDataConfig == nil {
		return nil, fmt.Errorf("pluginDataConfig is nil")
	}

	val, err := pluginDataConfig.Store.GetKey(apiID)
	if err != nil {
		return nil, fmt.Errorf("[+] could not get key '%s' from store: %s", apiID, err)
	}
	err = json.Unmarshal([]byte(val), &utterances)
	if err != nil {
		return nil, fmt.Errorf("[+] Error while unmarshalling the JSON object: %s", err)
	}

	return utterances, nil
}

func deleteApiUterances(apiID string, pluginDataConfig *PluginDataConfig) error {
	if pluginDataConfig == nil {
		return fmt.Errorf("pluginDataConfig is nil")
	}

	if result := pluginDataConfig.Store.DeleteKey(apiID); !result {
		return fmt.Errorf("Can't found apiID to delete: %w", apiID)
	}

	return nil
}
