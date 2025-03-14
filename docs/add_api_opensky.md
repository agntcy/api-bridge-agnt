# How to add a new API

Let's say you want to add a new API to your Tyk API Gateway, and make it available over natural language. Here are the steps:

- Create or get the OpenAPI specification for your API.
- Add the `x-tyk-api-gateway` section to your OpenAPI specification.
- Add the `x-nl-input-examples` section to your OpenAPI operations.
- Configure Tyk to use this new API.
- Query your new API with natural language questions.

## Let's choose an API

Let's choose https://openskynetwork.github.io/opensky-api.
There is not OpenAPI specification available so we create one.

<details>
<summary>OpenSky Network OpenAPI (Click to expand)</summary>

```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "OpenSky Network API",
    "description": "API for accessing flight tracking data from the OpenSky Network",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "https://opensky-network.org/api"
    }
  ],
  "paths": {
    "/states/all": {
      "get": {
        "operationId": "getStatesAll",
        "summary": "Get all state vectors",
        "description": "Retrieve any state vector of the OpenSky Network",
        "parameters": [
          {
            "name": "time",
            "in": "query",
            "schema": {
              "type": "integer"
            },
            "description": "Unix timestamp to retrieve states for"
          },
          {
            "name": "icao24",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "description": "ICAO24 transponder address in hex"
          },
          {
            "name": "lamin",
            "in": "query",
            "schema": {
              "type": "number",
              "format": "float"
            },
            "description": "Lower bound for latitude"
          },
          {
            "name": "lomin",
            "in": "query",
            "schema": {
              "type": "number",
              "format": "float"
            },
            "description": "Lower bound for longitude"
          },
          {
            "name": "lamax",
            "in": "query",
            "schema": {
              "type": "number",
              "format": "float"
            },
            "description": "Upper bound for latitude"
          },
          {
            "name": "lomax",
            "in": "query",
            "schema": {
              "type": "number",
              "format": "float"
            },
            "description": "Upper bound for longitude"
          },
          {
            "name": "extended",
            "in": "query",
            "schema": {
              "type": "integer"
            },
            "description": "Set to 1 to include aircraft category"
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StateVector"
                }
              }
            }
          }
        }
      }
    },
    "/states/own": {
      "get": {
        "operationId": "getStatesOwn",
        "summary": "Get own state vectors",
        "description": "Retrieve state vectors for authenticated user's sensors",
        "security": [
          {
            "basicAuth": []
          }
        ],
        "parameters": [
          {
            "name": "time",
            "in": "query",
            "schema": {
              "type": "integer"
            }
          },
          {
            "name": "icao24",
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "serials",
            "in": "query",
            "schema": {
              "type": "integer"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StateVector"
                }
              }
            }
          }
        }
      }
    },
    "/flights/all": {
      "get": {
        "operationId": "getFlightsAll",
        "summary": "Get flights in time interval",
        "parameters": [
          {
            "name": "begin",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            }
          },
          {
            "name": "end",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Flight"
                  }
                }
              }
            }
          }
        }
      }
    },
    "/flights/aircraft": {
      "get": {
        "operationId": "getFlightsAircraft",
        "summary": "Get flights by aircraft",
        "description": "Retrieve flights for a particular aircraft within a time interval",
        "parameters": [
          {
            "name": "icao24",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "Unique ICAO 24-bit address of the transponder in hex string representation (lower case)"
          },
          {
            "name": "begin",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            },
            "description": "Start of time interval as Unix timestamp (seconds since epoch)"
          },
          {
            "name": "end",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            },
            "description": "End of time interval as Unix timestamp (seconds since epoch)"
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Flight"
                  }
                }
              }
            }
          },
          "404": {
            "description": "No flights found for the given time period"
          }
        }
      }
    },
    "/flights/arrival": {
      "get": {
        "operationId": "getFlightsArrival",
        "summary": "Get arrivals by airport",
        "description": "Retrieve flights that arrived at a specific airport within a given time interval",
        "parameters": [
          {
            "name": "airport",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ICAO identifier for the airport"
          },
          {
            "name": "begin",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            },
            "description": "Start of time interval as Unix timestamp (seconds since epoch)"
          },
          {
            "name": "end",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            },
            "description": "End of time interval as Unix timestamp (seconds since epoch)"
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Flight"
                  }
                }
              }
            }
          },
          "404": {
            "description": "No flights found for the given time period"
          }
        }
      }
    },
    "/flights/departure": {
      "get": {
        "operationId": "getFlightsDeparture",
        "summary": "Get departures by airport",
        "description": "Retrieve flights that departed from a specific airport within a given time interval",
        "parameters": [
          {
            "name": "airport",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ICAO identifier for the airport (usually upper case)"
          },
          {
            "name": "begin",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            },
            "description": "Start of time interval as Unix timestamp (seconds since epoch)"
          },
          {
            "name": "end",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            },
            "description": "End of time interval as Unix timestamp (seconds since epoch)"
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Flight"
                  }
                }
              }
            }
          },
          "404": {
            "description": "No flights found for the given time period"
          }
        }
      }
    },
    "/tracks": {
      "get": {
        "operationId": "getTracks",
        "summary": "Get track by aircraft",
        "description": "Retrieve the trajectory for a certain aircraft at a given time. The trajectory is a list of waypoints\ncontaining position, barometric altitude, true track and an on-ground flag.\nNote: This endpoint is experimental.\n",
        "parameters": [
          {
            "name": "icao24",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "Unique ICAO 24-bit address of the transponder in hex string representation (lower case)"
          },
          {
            "name": "time",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            },
            "description": "Unix timestamp. Can be any time between start and end of a known flight. If time = 0, returns live track if there is any ongoing flight."
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Track"
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "StateVector": {
        "type": "object",
        "properties": {
          "time": {
            "type": "integer"
          },
          "states": {
            "type": "array",
            "items": {
              "type": "array",
              "items": {
                "oneOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "number"
                  },
                  {
                    "type": "boolean"
                  },
                  {
                    "type": "array",
                    "items": {
                      "type": "integer"
                    }
                  }
                ]
              }
            }
          }
        }
      },
      "Flight": {
        "type": "object",
        "properties": {
          "icao24": {
            "type": "string"
          },
          "firstSeen": {
            "type": "integer"
          },
          "lastSeen": {
            "type": "integer"
          },
          "callsign": {
            "type": "string"
          }
        }
      },
      "Track": {
        "type": "object",
        "properties": {
          "icao24": {
            "type": "string",
            "description": "Unique ICAO 24-bit address of the transponder in lower case hex string"
          },
          "startTime": {
            "type": "integer",
            "description": "Time of the first waypoint in seconds since epoch"
          },
          "endTime": {
            "type": "integer",
            "description": "Time of the last waypoint in seconds since epoch"
          },
          "callsign": {
            "type": "string",
            "nullable": true,
            "description": "Callsign (8 characters) that holds for the whole track"
          },
          "path": {
            "type": "array",
            "description": "Waypoints of the trajectory",
            "items": {
              "type": "array",
              "minItems": 6,
              "maxItems": 6,
              "items": {
                "oneOf": [
                  {
                    "type": "integer"
                  },
                  {
                    "type": "number"
                  },
                  {
                    "type": "number"
                  },
                  {
                    "type": "number"
                  },
                  {
                    "type": "number"
                  },
                  {
                    "type": "boolean"
                  }
                ]
              }
            }
          }
        }
      }
    },
    "securitySchemes": {
      "basicAuth": {
        "type": "http",
        "scheme": "basic"
      }
    }
  }
}
```

</details>

## We need to update the API in order to be configured inside the Tyk gateway

Let's add the `x-tyk-api-gateway` extension. You can find more information about
it in the [Tyk OAS API Definition](https://tyk.io/docs/api-management/gateway-config-tyk-oas/).

```json
  "x-tyk-api-gateway": {
    "info": {
      "id": "tyk-opensky-network-id",
      "name": "OpenSky Network API",
      "state": {
        "active": true
      }
    },
    "upstream": {
      "url": "https://opensky-network.org/api"
    },
    "server": {
      "listenPath": {
        "value": "/opensky/",
        "strip": true
      }
    },
    "middleware": {
      "global": {
        "pluginConfig": {
          "data": {
            "enabled": true,
            "value": {}
          },
          "driver": "goplugin"
        },
        "postPlugins": [
          {
            "enabled": true,
            "functionName": "SelectAndRewrite",
            "path": "middleware/agent-bridge-plugin.so"
          },
          {
            "enabled": true,
            "functionName": "RewriteQueryToOas",
            "path": "middleware/agent-bridge-plugin.so"
          }
        ],
        "responsePlugins": [
          {
            "enabled": true,
            "functionName": "RewriteResponseToNl",
            "path": "middleware/agent-bridge-plugin.so"
          }
        ]
      }
    }
  },
```

## Configure Tyk with this new API

```shell
curl http://localhost:8080/tyk/apis/oas \
  --header "x-tyk-authorization: foo" \
  --header 'Content-Type: text/plain' \
  -d@configs/opensky_network.json

curl http://localhost:8080/tyk/reload/group --header "x-tyk-authorization: foo"
```

- Tyk is now listening on the `/opensky/` path.
- You can now query the `/opensky/` path using a natural language query, by
  adding the `application/nlq` HTTP Content-Type header, and your query as body.

```shell
curl http://localhost:8080/opensky/ \
  --header 'Content-Type: application/nlq' \
  -d 'Get flights from 12pm to 1pm on March 11th 2025'

Here are the flights that were recorded between 12 PM and 1 PM on March 11th, 2025:

1. **Flight DESET** (ICAO24: 3d34ab)
   - Departure Airport: EDEN
   - Arrival Airport: EDVK
   - First Seen: 12:54 PM
   - Last Seen: 1:03 PM
   - Horizontal Distance from Departure Airport: 9,544 meters
   - Vertical Distance from Departure Airport: 1,181 meters

2. **Flight THD120** (ICAO24: 88530e)
   - Departure Airport: VTBS
   - Arrival Airport: Not specified
   - First Seen: 12:47 PM
   - Last Seen: 12:54 PM
   - Horizontal Distance from Departure Airport: 2,237 meters
   - Vertical Distance from Departure Airport: 432 meters

[...]
```

Here is what happened behind the scenes:

1. **Content-Type Detection**: The system recognizes the `application/nlq` content type
2. **Operation Matching**:
   - Compares your query ("Get flights from 12pm to 1pm on March 11th 2025")
     against the `x-nl-input-examples` or the operation description
   - Identifies the closest matching operation (`getFlightsAll` in this case, ie `GET /flights/all`)
3. **Parameter Extraction**:
   - An LLM extracts relevant parameters from your query
   - Converts time references to Unix timestamps
   - Builds a proper API request
4. **Request Transformation**:
   - Converts the natural query to a proper HTTP request
   - Forwards the request to the upstream API (`GET /flights/all?start=1678560000&end=1678563600` for example)
5. **Response Handling**:
   - Receives the raw API response
   - Returns the JSON response with the flight data

Without any code you were able to query the OpenSky Network API and retrieve flight data.

## Improving the operation matching

When the `description` field provided in the operation definition are poorly
written or missing, it's possible to improve the selection of the operation by
providing examples of queries that should match the operation.

This is done by adding the `x-nl-input-examples` field to the operation definition.

For example, you could provide the following examples which could help improve the operation matching:

In the OpenAPI specification, add the `x-nl-output-examples` field to the operation definition:

```json
[...]
"paths": {
  [...]
  "/flights/all": {
    "get": {
      "operationId": "getFlightsAll",
      "summary": "Get flights in time interval",
      "x-nl-input-examples": [
         "Get flights from 12pm to 1pm on March 11th 2025",
         "Donne moi les vols de 12h à 13h le 11 mars 2025",
         "List of flights from 12pm to 1pm on March 11th 2025"
      ]
      "parameters": [
        {
          "name": "begin",
          "in": "query",
          "required": true,
          "schema": {
            "type": "integer"
          }
        },
        {
          "name": "end",
          "in": "query",
          "required": true,
          "schema": {
            "type": "integer"
          }
        }
      ],
      "responses": {
        "200": {
          "description": "Successful response",
          "content": {
            "application/json": {
              "schema": {
                "type": "array",
                "items": {
                  "$ref": "#/components/schemas/Flight"
                }
              }
            }
          }
        }
      }
    }
  },

```
