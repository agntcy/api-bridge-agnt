// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
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
