// Package contractassets exposes the canonical, versioned API contract files
// as immutable process assets. Embedding keeps the container response identical
// to the reviewed source without runtime filesystem or network access.
package contractassets

import _ "embed"

// Version is shared by the canonical OpenAPI document, developer metadata,
// and the stable download filename. Contract tests prevent version drift.
const Version = "2.1.0"

// OpenAPIYAML returns the canonical private API description.
//
//go:embed openapi.yaml
var openAPIYAML string

func OpenAPIYAML() string { return openAPIYAML }

// DeveloperExamplesV1 returns the bounded, non-secret developer workspace source.
//
//go:embed developer-examples.v1.json
var developerExamplesV1 string

func DeveloperExamplesV1() string { return developerExamplesV1 }
