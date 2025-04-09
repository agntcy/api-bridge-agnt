// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func dump(title string, any interface{}) {
	prettyPrintStruct, err := PrettyPrintStruct(any)
	if err != nil {
		logger.Fatalf("[+] failed to dump %v: err=%v", any, err)
		return
	}
	fmt.Printf("%v:\n%v\n", title, prettyPrintStruct)
}

func PrettyPrintStruct(v any) (string, error) {
	configJSON, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal struct: %v, err=%v", v, err)
	}
	return string(configJSON), nil
}

func dumpPluginConfig() {
	pluginConfigLock.RLock()
	defer pluginConfigLock.RUnlock()

	filePath := "./testdata/config_save.json"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		logger.Fatalf("[+] failed to open file: %v, err=%v", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(pluginConfig)
	if err != nil {
		logger.Fatalf("[+] failed to dump plugin config: err=%v", err)
	}
}

func dumpStructIntoJson(filePath string, any interface{}) {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		logger.Fatalf("[+] failed to open file: %v, err=%v", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(any)
	if err != nil {
		logger.Fatalf("[+] failed to dump struct into json: err=%v", err)
	}
}
