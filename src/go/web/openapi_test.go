package web

import (
	"context"
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
}
