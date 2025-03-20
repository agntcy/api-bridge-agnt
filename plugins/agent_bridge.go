// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/TykTechnologies/tyk/ctx"
	"github.com/TykTechnologies/tyk/log"
	"github.com/TykTechnologies/tyk/user"

	"github.com/getkin/kin-openapi/routers"
)

const (
	CONTENT_TYPE_NLQ          = "application/nlq"
	HEADER_X_NL_QUERY_ENABLED = "X-Nl-Query-Enabled"
	HEADER_X_NL_RESPONSE_TYPE = "X-Nl-Response-Type"

	RESPONSE_TYPE_NL       = "nl"       // Rewrite the response to Natural Language
	RESPONSE_TYPE_UPSTREAM = "upstream" // Keep the response as it is

	INTERNAL_ERROR_MSG = "I'm sorry, but I wasn't able to process your request, it's an internal error"
)

const (
	RELEVANCE_THRESHOLD = 0.5
)

const (
	METADATA_NLQ           = "NLQuery"
	METADATA_RESPONSE_TYPE = "ResponseType"
)

var logger = log.Get()

func SelectAndRewrite(rw http.ResponseWriter, r *http.Request) {
	logger.Debugf("[+] Inside SelectAndRewrite ...")
	apiConfig, err := initPluginFromRequest(r)
	if err != nil {
		logger.Debugf("[+] Failed to init plugin from request: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
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

	matchingOperation, matchingScore, err := findSelectOperation(apiConfig.APIID, nlq)
	if err != nil {
		logger.Errorf("[+] Error while selecting operation: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return

	} else if matchingOperation == nil || (matchingScore < RELEVANCE_THRESHOLD) {
		logger.Debugf("[+] No matching operation found")
		http.Error(rw, "No matching operation found", http.StatusNotFound)
		return
	}
	logger.Debugf("[+] Selected endpoint: %#v - %#v", *matchingOperation, matchingScore)

	apidef := getOASDefinition(r)
	if apidef == nil {
		err := fmt.Errorf("API definition is nil")
		logger.Errorf("[+] SelectAndRewrite: %s", err)
		return
	}

	// Iterate through all paths and operations in the API definition
	for path, pathItem := range apidef.Paths {
		for method, operation := range pathItem.Operations() {
			if operation.OperationID == *matchingOperation {
				route := &routers.Route{
					Path:      path,
					Method:    method,
					Operation: operation,
				}
				emptyPathParams := map[string]string{}
				r.URL.Path = path
				r.Method = method
				err := rewriteQueryForRoute(r, route, emptyPathParams)
				if err != nil {
					logger.Errorf("[+] Error rewriting the query: %s", err)
					http.Error(rw, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
	}
}

func RewriteQueryToOas(rw http.ResponseWriter, r *http.Request) {
	_, err := initPluginFromRequest(r)
	if err != nil {
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	if !shouldRewriteQuery(r) {
		logger.Debugf("[+] We were not asked to rewrite the query, ignoring ...")
		r.Header.Del(HEADER_X_NL_QUERY_ENABLED)
		return
	}
	r.Header.Del(HEADER_X_NL_QUERY_ENABLED)

	// Save useful information in the session in order to be able to rewrite the response
	nlSentence, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("[+] Error while reading the body: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}
	session := &user.SessionState{
		MetaData: map[string]any{
			METADATA_NLQ:           string(nlSentence),
			METADATA_RESPONSE_TYPE: r.Header.Get(HEADER_X_NL_RESPONSE_TYPE),
		},
	}
	r.Header.Del(HEADER_X_NL_RESPONSE_TYPE)
	ctx.SetSession(r, session, true)

	logger.Debug("[+] Rewriting Natural language query ...")

	err = rewriteQuery(r)
	if err != nil {
		logger.Errorf("[+] Error rewriting the query: %s", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
}

func RewriteResponseToNl(rw http.ResponseWriter, res *http.Response, req *http.Request) {
	_, err := initPluginFromRequest(req)
	if err != nil {
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	if !shouldRewriteResponseToNl(req) {
		logger.Debugf("[+] We were not asked to rewrite the response, ignoring ...")
		return
	}

	logger.Debug("[+] Rewriting response to Natural language ...")

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Errorf("[+] Error while reading response body: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	if res.Header.Get("Content-Encoding") == "gzip" {
		bodyBytes, err = GetUnzipContent(bodyBytes)
		if err != nil {
			logger.Errorf("[+] Error while unzipping the response body: %s", err)
			http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
			return
		}
		res.Header.Del("Content-Encoding")
	}

	naturalLanguageResponse, err := responseToNL(req, fmt.Sprintf("%s %s", res.Status, string(bodyBytes)))
	if err != nil {
		logger.Errorf("[+] Error while converting the response to Natural Language: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	res.StatusCode = http.StatusOK

	res.Header.Set("Content-Type", "text/plain; charset=utf-8")
	res.Header.Set("Content-Length", fmt.Sprint(len(naturalLanguageResponse)))

	res.Body = io.NopCloser(strings.NewReader(naturalLanguageResponse))
	res.ContentLength = int64(len(naturalLanguageResponse))
}

func QueryEndpointSelection(rw http.ResponseWriter, r *http.Request) {
	logger.Debugf("[+] Entering QueryEndpointSelection ...")

	apiConfig, err := initPluginFromRequest(r)
	if err != nil {
		logger.Debugf("[+] Failed to init plugin from request: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	queries := r.URL.Query()
	selectionQueries, present := queries["query"]
	if !present {
		logger.Debugf("[+] Failed to find \"query\" query parameter")
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	var replyData QueryEndpointSelectionReply

	if len(selectionQueries) > 0 {
		matches := selectEndpoint(apiConfig.APIID, selectionQueries)
		replyData = QueryEndpointSelectionReply{
			Results: matches,
		}
	} else {
		replyData = QueryEndpointSelectionReply{
			Results: []QueryEndpointSelectionMatch{},
		}
	}

	jsonData, err := json.Marshal(replyData)
	if err != nil {
		logger.Debugf("[+] Failed to marshal JSON reply data: %s", err)
		http.Error(rw, INTERNAL_ERROR_MSG, http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(jsonData)
}

func init() {
	logger.Infof("[+] Initializing API Bridge Agnt plugin ...")
}

func main() {}
