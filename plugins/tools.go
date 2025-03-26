// Copyright AGNTCY Contributors (https://github.com/agntcy)
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/TykTechnologies/tyk/apidef/oas"
	"github.com/TykTechnologies/tyk/ctx"
)

func GetUnzipContent(zipContent []byte) ([]byte, error) {
	buf := bytes.NewBuffer(zipContent)
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	unzippedContent, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return unzippedContent, nil
}

/*
Note: This is a workaround for an issue in Tyk function ctx.GetOASDefinition(r) that do a "Reflect.Clone(val)".
This cause a crash with a stack overflow error when using a spec with recursive references, like JIRA one.
*/
func getOASDefinition(r *http.Request) *oas.OAS {
	if v := r.Context().Value(ctx.OASDefinition); v != nil {
		if val, ok := v.(*oas.OAS); ok {
			return val
		}
	}
	return nil
}
