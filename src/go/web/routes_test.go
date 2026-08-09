package web

import (
	"net/http"
	"testing"

	"github.com/gorilla/mux"
)

func TestExpandedRoutes(t *testing.T) {
	t.Parallel()

	router := mux.NewRouter()
	addExpandedRoutes(router)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/experiments/example/apps/input"},
		{method: http.MethodPost, path: "/experiments/example/reconfigure"},
		{method: http.MethodPost, path: "/experiments/example/vms/node/connect"},
		{method: http.MethodPost, path: "/experiments/example/vms/node/disconnect"},
		{method: http.MethodPost, path: "/experiments/example/vms/node/resetDisk"},
		{method: http.MethodGet, path: "/experiments/example/vlans/aliases"},
		{method: http.MethodPost, path: "/experiments/example/vlans/aliases"},
		{method: http.MethodGet, path: "/experiments/example/vlans/ranges"},
		{method: http.MethodPost, path: "/experiments/example/vlans/ranges"},
		{method: http.MethodPost, path: "/disks/inject"},
		{method: http.MethodGet, path: "/images/example/build"},
		{method: http.MethodPost, path: "/images/example/build"},
		{method: http.MethodGet, path: "/vlans"},
		{method: http.MethodGet, path: "/schedulers"},
		{method: http.MethodPost, path: "/roles"},
		{method: http.MethodGet, path: "/roles/example"},
		{method: http.MethodPatch, path: "/roles/example"},
		{method: http.MethodDelete, path: "/roles/example"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()

			request, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}

			var match mux.RouteMatch
			if !router.Match(request, &match) {
				t.Fatalf("route not registered: %v", match.MatchErr)
			}
		})
	}
}
