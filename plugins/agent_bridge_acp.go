// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/TykTechnologies/tyk/ctx"
	"github.com/TykTechnologies/tyk/user"
)

// SetContext updates the context of a request.
func SetContext(r *http.Request, ctx context.Context) {
	r2 := r.WithContext(ctx)
	*r = *r2
}

func ProcessACPQuery(rw http.ResponseWriter, r *http.Request) {
	logger.Debug("[+] ###AXT:: ProcessACPQuery -->")

	if len(acpPluginData.ACPPluginServices) == 0 {
		err := initACPPluginApiConfig(r)
		if err != nil {
			logger.Errorf("[+] Error while getting the ACP plugin config: %s", err)
			http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
			return
		}
	}

	// POST and Content-Type: application/nlq are expected
	if !(r.Method == "POST" && r.Header.Get("Content-Type") == CONTENT_TYPE_NLQ) {
		logger.Debugf("[+] Query is not POST or Content-Type is not %s, ignoring ...", CONTENT_TYPE_NLQ)
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

	service, err := findACPServiceFromQuery(nlq)
	if err != nil {
		logger.Errorf("[+] Failed to find a service for query: %s", nlq)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}
	logger.Infof("[+] ###AXT:: found a service (%v) to reach for query=%v", service, nlq)

	u, err := url.Parse(service)
	if err != nil {
		logger.Errorf("[+] Error while parsing the service URL (%v): %s", service, err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}
	logger.Infof("[+] ###AXT:: UrlRewriteTarget, u= (%v) ", u)

	rctx := r.Context()
	rctx = context.WithValue(rctx, ctx.UrlRewriteTarget, u)
	SetContext(r, rctx)

	logger.Debug("[+] ###AXT:: ProcessACPQuery <--")
}
