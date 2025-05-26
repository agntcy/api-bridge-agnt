// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"net/http"
)

type MCPToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type MCPServerInfo struct {
	Name  string        `json:"name"`
	Tools []MCPToolInfo `json:"tools"`
}

type OpenAPIServiceInfo struct {
	Name string `json:"name"`
}

type Info struct {
	Version         string                        `json:"version"`
	MCPServers      map[string]MCPServerInfo      `json:"mcp_servers"`
	OpenAPIServices map[string]OpenAPIServiceInfo `json:"openapi_services"`
}

func processInfo(rw http.ResponseWriter, r *http.Request) {
	info := Info{
		Version:         "1.0.0",
		MCPServers:      map[string]MCPServerInfo{},
		OpenAPIServices: map[string]OpenAPIServiceInfo{},
	}

	for mcpServerName, mcpServer := range mcpConfig {
		name := mcpServerName
		mcpServerInfo := MCPServerInfo{
			Name:  name,
			Tools: []MCPToolInfo{},
		}
		for _, tool := range mcpServer.Tools {
			mcpServerInfo.Tools = append(mcpServerInfo.Tools, MCPToolInfo{Name: tool.Name, Description: tool.Description})
		}
		info.MCPServers[name] = mcpServerInfo
	}

	for service := range servicePluginData.PluginServices {
		serviceName := service

		openAPIServiceInfo := OpenAPIServiceInfo{
			Name: serviceName,
		}
		info.OpenAPIServices[serviceName] = openAPIServiceInfo
	}

	response, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		logger.Errorf("[+] Error retrieving info about API Bridge Agent: %s", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(response)
}
