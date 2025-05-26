// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/TykTechnologies/tyk/config"
	"github.com/TykTechnologies/tyk/storage"
)

const (
	AGENT_BRIDGE_DEFAULT_KEY_PREFIX = "agent_bridge:"
	AGENT_BRIDGE_DEFAULT_TTL        = -1
)

var agentBridgeStore *storage.RedisCluster
var storeVersion int64 = 0
var storeVersionLock = &sync.RWMutex{}

func safeIncrementStoreVersion() int64 {
	// Increment the store version safely. The increment operator is not atomic, so we need to use a lock.
	storeVersionLock.Lock()
	defer storeVersionLock.Unlock()
	storeVersion++
	return storeVersion
}

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
	if agentBridgeStore == nil {
		return fmt.Errorf("storage is not configured")
	}

	utterances := []string{}
	for _, aiExtention := range pluginDataConfig.SelectOperations {
		utterances = append(utterances, aiExtention.InputExamples...)
	}
	apiConfig := apiServicePluginApiConfig{
		APIName:    pluginDataConfig.APIID,
		Target:     fmt.Sprintf("tyk://%s%s", apiID, pluginDataConfig.ListenPath),
		Utterances: utterances,
	}

	jsonApiConfig, err := json.Marshal(apiConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal struct: %v, err=%v", utterances, err)
	}

	logger.Debugf("[+] Save config to redis key : %s", apiID)
	// Save the plugin data config to the Redis store
	if err := agentBridgeStore.SetKey(apiID, string(jsonApiConfig), AGENT_BRIDGE_DEFAULT_TTL); err != nil {
		return fmt.Errorf("failed to save plugin data config: %w", err)
	}

	storeVersion = safeIncrementStoreVersion()
	logger.Debugf("[+] New store version=%d", storeVersion)

	return nil
}

func deleteApiUterances(apiID string, pluginDataConfig *PluginDataConfig) error {
	if pluginDataConfig == nil {
		return fmt.Errorf("pluginDataConfig is nil")
	}
	if agentBridgeStore == nil {
		return fmt.Errorf("storage is not configured")
	}

	logger.Debugf("[+] Delete redis key : %s", apiID)
	if result := agentBridgeStore.DeleteKey(apiID); !result {
		return fmt.Errorf("can't found apiID to delete: %s", apiID)
	}

	storeVersion = safeIncrementStoreVersion()
	logger.Debugf("[+] New store version=%d", storeVersion)

	return nil
}
