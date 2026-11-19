// Package docs embeds the OpenAPI specification of the Panel API.
package docs

import _ "embed"

// OpenAPI is the OpenAPI 3 specification served at /swagger/openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPI []byte
