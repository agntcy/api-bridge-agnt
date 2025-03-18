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
			"",
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
			"Operation summary: The request's query parameters.\n",
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
			"Operation description: The request's query parameters, with a description.\n",
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
			"Operation description: The request's query parameters, with a description.\n",
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
			`Operation summary: The request's query parameters.
The list of Parameters:
- {"in":"query","name":"genre"}
- {"description":"The age of the person","in":"header","name":"age","schema":{"$ref":"#/components/schemas/age0"}}
- {"description":"The age of the person","in":"header","name":"name","schema":{"type":"integer"}}
The list of References:
- #/components/schemas/age: {"format":"int32","type":"integer"}
- #/components/schemas/age0: {"properties":{"age":{"$ref":"#/components/schemas/age"},"romanage":{"type":"string"}},"type":"object"}
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
			`Operation summary: The request's query parameters.
The request body:
{"properties":{"url":{"type":"string"}},"required":["url"],"type":"object"}
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
			`Operation summary: The request's query parameters.
The list of Parameters:
- {"in":"query","name":"genre"}
- {"description":"The age of the person","in":"header","name":"age","schema":{"$ref":"#/components/schemas/age0"}}
- {"description":"The age of the person","in":"header","name":"name","schema":{"type":"integer"}}
The request body:
{"properties":{"age":{"$ref":"#/components/schemas/age"},"city":{"$ref":"#/components/schemas/city"},"url":{"type":"string"}},"required":["url"],"type":"object"}
The list of References:
- #/components/schemas/age: {"format":"int32","type":"integer"}
- #/components/schemas/age0: {"properties":{"age":{"$ref":"#/components/schemas/age"},"romanage":{"type":"string"}},"type":"object"}
- #/components/schemas/city: {"type":"string"}
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
			var md *openapi3.MediaType
			if operation.RequestBody != nil {
				md = operation.RequestBody.Value.GetMediaType("application/json")
			}
			got, _ := buildOperationString(operation, md)
			assert.Equal(t, tt.expected, got)
		})
	}
}
