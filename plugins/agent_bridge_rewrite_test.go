// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
)

func TestBuildOperationString(t *testing.T) {
	tests := []struct {
		description string
		spec        []byte
		expected    string
	}{
		{
			"An empty operation",
			[]byte(`{
        "openapi": "3.0.0",
        "info": {
          "title": "Minimal API",
          "version": "1.0.0"
        },
        "paths": {
          "/test": {
            "get": {}
          }
        }
      }`),
			`{"responses":null}
`,
		},
		{
			"Simple operation with summary",
			[]byte(`{
        "openapi": "3.0.0",
        "info": {
          "title": "Minimal API",
          "version": "1.0.0"
        },
        "paths": {
          "/test": { "get": { "summary": "The request's query parameters." } }
        }
      }`),
			`{"responses":null,"summary":"The request's query parameters."}
`,
		},
		{
			"Simple operation with description",
			[]byte(`{
        "openapi": "3.0.0",
        "info": {
          "title": "Minimal API",
          "version": "1.0.0"
        },
        "paths": {
          "/test": { "get": { "description": "The request's query parameters, with a description." } }
        }
      }`),
			`{"description":"The request's query parameters, with a description.","responses":null}
`,
		},
		{
			"Simple operation with description and summary, prefer description",
			[]byte(`{
        "openapi": "3.0.0",
        "info": {
          "title": "Minimal API",
          "version": "1.0.0"
        },
        "paths": {
          "/test": { "get": { "description": "The request's query parameters, with a description.", "summary": "The request's query parameters summary." } }
        }
      }`),
			`{"description":"The request's query parameters, with a description.","responses":null,"summary":"The request's query parameters summary."}
`,
		},
		{
			"An operation with a parameter using a $ref",
			[]byte(`{
        "openapi": "3.0.0",
        "info": {
          "title": "Minimal API",
          "version": "1.0.0"
        },
        "paths": {
          "/test": {
          	"get": {
           		"summary": "The request's query parameters.",
             	"parameters": [
             		{ "name": "genre", "in": "query" },
               		{ "$ref": "#/components/parameters/age" },
                 	{ "$ref": "#/components/parameters/name" }
                  ]
            }
          }
        },
        "components": {
          "schemas": {
            "age0": {
              "$ref": "#/components/schemas/age1"
            },
            "age1": {
              "$ref": "#/components/schemas/age2"
            },
            "age2": {
              "$ref": "#/components/schemas/age3"
            },
            "age3": {
              "$ref": "#/components/schemas/age4"
            },
            "age4": {
              "type": "object",
              "properties": {
                "romanage": {
                  "type": "string"
                },
                "age": {
                  "$ref": "#/components/schemas/age"
                }
              }
            },
            "age": {
              "type": "integer",
              "format": "int32"
            }
          },
          "parameters": {
            "age": {
              "name": "age",
              "in": "header",
              "description": "The age of the person",
              "schema": {
                "$ref": "#/components/schemas/age0"
              }
            },
            "name": {
              "name": "name",
              "in": "header",
              "description": "The age of the person",
              "schema": {
                "type": "integer"
              }
            }
          }
        }
      }`),
			`{"parameters":[{"in":"query","name":"genre"},{"$ref":"#/components/parameters/age"},{"$ref":"#/components/parameters/name"}],"responses":null,"summary":"The request's query parameters."}
The list of References:
===
- #/components/schemas/age: {"format":"int32","type":"integer"}
- #/components/schemas/age0: {"properties":{"age":{"$ref":"#/components/schemas/age"},"romanage":{"type":"string"}},"type":"object"}
===
`,
		},
		{
			"Simple operation - request body",
			[]byte(`{
		"openapi": "3.0.0",
        "info": {
          "title": "Minimal API",
          "version": "1.0.0"
        },
        "paths": {
          "/test": {
          	"get": {
			  "summary": "The request's query parameters.",
			  "requestBody": {
			    "content": {
			      "application/json": {
			        "schema": {
			          "required": [
			            "url"
			          ],
			          "type": "object",
			          "properties": {
			            "url": {
			              "type": "string"
			            }
			          }
			        }
			      }
			    },
			    "required": true
			  }
			}
		  }
        }
	  }
		 `),
			`{"requestBody":{"content":{"application/json":{"schema":{"properties":{"url":{"type":"string"}},"required":["url"],"type":"object"}}},"required":true},"responses":null,"summary":"The request's query parameters."}
`,
		},
		{
			"Simple operation - params, request body, with $refs ",
			[]byte(`{
				"openapi": "3.0.0",
        "info": {
          "title": "Minimal API",
          "version": "1.0.0"
        },
        "paths": {
          "/test": {
          	"get": {
			  "summary": "The request's query parameters.",
              "parameters": [
                { "name": "genre", "in": "query" },
               	{ "$ref": "#/components/parameters/age" },
               	{ "$ref": "#/components/parameters/name" }
              ],
			  "requestBody": {
			    "content": {
			      "application/json": {
			        "schema": {
			          "required": [
			            "url"
			          ],
			          "type": "object",
			          "properties": {
			            "url": { "type": "string" },
						"age": { "$ref": "#/components/schemas/age" },
						"city": { "$ref": "#/components/schemas/city" }
			          }
			        }
			      }
			    },
			    "required": true
			  }
			}
		  }
		},
		"components": {
          "schemas": {
            "age0": {
              "$ref": "#/components/schemas/age1"
            },
            "age1": {
              "$ref": "#/components/schemas/age2"
            },
            "age2": {
              "$ref": "#/components/schemas/age3"
            },
            "age3": {
              "$ref": "#/components/schemas/age4"
            },
            "age4": {
              "type": "object",
              "properties": {
                "romanage": {
                  "type": "string"
                },
                "age": {
                  "$ref": "#/components/schemas/age"
                }
              }
            },
            "age": {
              "type": "integer",
              "format": "int32"
            },
            "city": {
            	"type": "string"
            }
          },
          "parameters": {
            "age": {
              "name": "age",
              "in": "header",
              "description": "The age of the person",
              "schema": {
                "$ref": "#/components/schemas/age0"
              }
            },
            "name": {
              "name": "name",
              "in": "header",
              "description": "The age of the person",
              "schema": {
                "type": "integer"
              }
            }
          }
        }
	  }
				 `),
			`{"parameters":[{"in":"query","name":"genre"},{"$ref":"#/components/parameters/age"},{"$ref":"#/components/parameters/name"}],"requestBody":{"content":{"application/json":{"schema":{"properties":{"age":{"$ref":"#/components/schemas/age"},"city":{"$ref":"#/components/schemas/city"},"url":{"type":"string"}},"required":["url"],"type":"object"}}},"required":true},"responses":null,"summary":"The request's query parameters."}
The list of References:
===
- #/components/schemas/age: {"format":"int32","type":"integer"}
- #/components/schemas/age0: {"properties":{"age":{"$ref":"#/components/schemas/age"},"romanage":{"type":"string"}},"type":"object"}
- #/components/schemas/city: {"type":"string"}
===
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			loader := openapi3.NewLoader()
			doc, err := loader.LoadFromData([]byte(tt.spec))
			if err != nil {
				t.Fatalf("Invalid OpenAPI specification, there is a problem in the tests: %s", err)
			}
			operation := doc.Paths["/test"].Get
			got, _ := buildOperationString(operation)
			assert.Equal(t, tt.expected, got)
		})
	}
}
