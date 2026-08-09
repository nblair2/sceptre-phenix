package web

import (
	"context"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPI(t *testing.T) {
	t.Parallel()

	document, err := openapi3.NewLoader().LoadFromFile("public/docs/openapi.yml")
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}

	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI document: %v", err)
	}

	documented := make(map[string]*openapi3.PathItem)
	for path, item := range document.Paths.Map() {
		documented[canonicalPath(path)] = item
	}

	for route := range routeSetFromText(t, expectedAPIRoutes) {
		method, path, ok := strings.Cut(route, " ")
		if !ok {
			t.Fatalf("invalid expected route: %s", route)
		}

		item := documented[canonicalPath(path)]
		if item == nil || item.GetOperation(method) == nil {
			t.Errorf("route is not documented: %s", route)
		}
	}
}

func canonicalPath(path string) string {
	var result strings.Builder

	for {
		start := strings.IndexByte(path, '{')
		if start == -1 {
			result.WriteString(path)
			break
		}

		result.WriteString(path[:start])
		end := strings.IndexByte(path[start:], '}')
		if end == -1 {
			result.WriteString(path[start:])
			break
		}

		result.WriteString("{}")
		path = path[start+end+1:]
	}

	return result.String()
}
