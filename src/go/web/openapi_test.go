package web

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gorilla/mux"
)

var errRoutesInspected = errors.New("routes inspected")

func TestOpenAPI(t *testing.T) {
	document, err := openapi3.NewLoader().LoadFromFile("public/docs/openapi.yml")
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}

	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI document: %v", err)
	}

	registered := registeredAPIRoutes(t)
	documented := documentedAPIRoutes(document)

	for route := range registered {
		if !documented[route] {
			t.Errorf("route is not documented: %s", route)
		}
	}

	for route := range documented {
		if !registered[route] {
			t.Errorf("documented route is not registered: %s", route)
		}
	}
}

func registeredAPIRoutes(t *testing.T) map[string]bool {
	t.Helper()

	routes := make(map[string]bool)
	err := start(func(router *mux.Router) error {
		err := router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
			path, err := route.GetPathTemplate()
			if err != nil {
				return err
			}

			methods, err := route.GetMethods()
			if err != nil {
				return err
			}

			path = strings.TrimPrefix(path, "/api/v1")
			for _, method := range methods {
				if method != "OPTIONS" {
					routes[method+" "+canonicalPath(path)] = true
				}
			}

			return nil
		})
		if err != nil {
			return err
		}

		return errRoutesInspected
	}, ServeUnbundled(), ServeWithUsers(nil), ServeWithFeatures([]string{"vm-mount"}))
	if !errors.Is(err, errRoutesInspected) {
		t.Fatalf("inspect registered API routes: %v", err)
	}

	return routes
}

func documentedAPIRoutes(document *openapi3.T) map[string]bool {
	routes := make(map[string]bool)
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "CONNECT", "OPTIONS", "TRACE"}

	for path, item := range document.Paths {
		for _, method := range methods {
			if item.GetOperation(method) != nil {
				routes[method+" "+canonicalPath(path)] = true
			}
		}
	}

	return routes
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
