package web

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

const expectedAPIRoutes = `
GET /builder/topologies
GET /builder/topologies/{name}
GET /configs
POST /configs
GET /configs/{kind}/{name}
PUT /configs/{kind}/{name}
DELETE /configs/{kind}/{name}
POST /configs/download
GET /schemas/{version}
GET /schemas/{kind}/{version}
GET /experiments
POST /experiments
POST /experiments/builder
PUT /experiments/builder
GET /experiments/{name}
PATCH /experiments/{name}
DELETE /experiments/{name}
GET /experiments/{name}/apps
GET /experiments/{name}/apps/input
POST /experiments/{name}/reconfigure
POST /experiments/{name}/start
POST /experiments/{name}/stop
GET /experiments/{exp}/netflow
POST /experiments/{exp}/netflow
DELETE /experiments/{exp}/netflow
GET /experiments/{exp}/netflow/ws
GET /experiments/{name}/topology
GET /experiments/{name}/topology/search
POST /experiments/{name}/trigger
DELETE /experiments/{name}/trigger
GET /experiments/{name}/schedule
POST /experiments/{name}/schedule
GET /experiments/{name}/captures
POST /experiments/{exp}/captureSubnet
POST /experiments/{exp}/stopCaptureSubnet
GET /experiments/{name}/files
GET /experiments/{name}/files/{filename}
GET /experiments/{name}/scorch/components/{run}/{loop}/{stage}/{cmp}
GET /experiments/{name}/scorch/components/{run}/{loop}/{stage}/{cmp}/ws
GET /experiments/{name}/scorch/pipelines
GET /experiments/{name}/scorch/pipelines/{run}/{loop}
POST /experiments/{name}/scorch/pipelines/{run}
DELETE /experiments/{name}/scorch/pipelines/{run}
GET /experiments/{name}/scorch/terminals
GET /experiments/{name}/scorch/terminals/{pid}
POST /experiments/{name}/scorch/terminals/{pid}/exit/{id}
GET /experiments/{name}/scorch/terminals/{pid}/ws/{id}
GET /experiments/{name}/scorch/terminals/{run}/{loop}/{stage}/{cmp}
GET /experiments/{name}/soh
GET /experiments/{name}/vlans/aliases
POST /experiments/{name}/vlans/aliases
GET /experiments/{name}/vlans/ranges
POST /experiments/{name}/vlans/ranges
GET /experiments/{exp}/vms
PATCH /experiments/{exp}/vms
GET /experiments/{exp}/vms/{name}
PATCH /experiments/{exp}/vms/{name}
DELETE /experiments/{exp}/vms/{name}
GET /experiments/{exp}/vms/{name}/reset
GET /experiments/{exp}/vms/{name}/restart
POST /experiments/{exp}/vms/{name}/start
POST /experiments/{exp}/vms/{name}/stop
GET /experiments/{exp}/vms/{name}/shutdown
POST /experiments/{exp}/vms/{name}/redeploy
POST /experiments/{exp}/vms/{name}/connect
POST /experiments/{exp}/vms/{name}/disconnect
POST /experiments/{exp}/vms/{name}/resetDisk
POST /experiments/{exp}/vms/{name}/cdrom
DELETE /experiments/{exp}/vms/{name}/cdrom
GET /experiments/{exp}/vms/{name}/screenshot.png
GET /experiments/{exp}/vms/{name}/vnc
GET /experiments/{exp}/vms/{name}/vnc/ws
GET /experiments/{exp}/vms/{name}/captures
POST /experiments/{exp}/vms/{name}/captures
DELETE /experiments/{exp}/vms/{name}/captures
GET /experiments/{exp}/vms/{name}/snapshots
POST /experiments/{exp}/vms/{name}/snapshots
POST /experiments/{exp}/vms/{name}/snapshots/{snapshot}
POST /experiments/{exp}/vms/{name}/commit
POST /experiments/{exp}/vms/{name}/memorySnapshot
GET /experiments/{exp}/vms/{name}/forwards
POST /experiments/{exp}/vms/{name}/forwards
DELETE /experiments/{exp}/vms/{name}/forwards
GET /experiments/{exp}/vms/{name}/forwards/{host}/{port}/ws
POST /experiments/{exp}/vms/{name}/mount
DELETE /experiments/{exp}/vms/{name}/unmount
GET /experiments/{exp}/vms/{name}/files
GET /experiments/{exp}/vms/{name}/files/download
PUT /experiments/{exp}/vms/{name}/files/upload
POST /experiments/{exp}/vms/{name}/files/copy
GET /disks
POST /disks/inject
GET /images/{name}/build
POST /images/{name}/build
POST /disks/snapshot
POST /disks/rebase
POST /disks/resize
POST /disks/commit
POST /disks/clone
DELETE /disks
POST /disks/rename
POST /disks
GET /disks/download
GET /vms
GET /applications
GET /topologies
GET /topologies/{topo}/scenarios
GET /hosts
GET /vlans
GET /schedulers
GET /users
POST /users
GET /users/{username}
PATCH /users/{username}
DELETE /users/{username}
POST /users/{username}/tokens
GET /roles
POST /roles
GET /roles/{name}
PATCH /roles/{name}
DELETE /roles/{name}
POST /signup
GET /login
POST /login
GET /logout
GET /logs
GET /ws
POST /console
GET /console/{pid}/ws
POST /console/{pid}/size
GET /settings
POST /settings
GET /settings/password
POST /workflow/apply/{branch}
POST /workflow/configs/{branch}
GET /options
`

func TestAPIRoutes(t *testing.T) {
	t.Parallel()

	expected := routeSetFromText(t, expectedAPIRoutes)
	actual := registeredAPIRoutes(t)

	for route := range expected {
		if !actual[route] {
			t.Errorf("route not registered: %s", route)
		}
	}

	for route := range actual {
		if !expected[route] {
			t.Errorf("unexpected route registered: %s", route)
		}
	}
}

func registeredAPIRoutes(t *testing.T) map[string]bool {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "server.go", nil, 0)
	if err != nil {
		t.Fatalf("parse server routes: %v", err)
	}

	routes := make(map[string]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		if literal, ok := node.(*ast.CompositeLit); ok {
			addRouteLiteral(routes, literal)
		}

		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		methods, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || methods.Sel.Name != "Methods" {
			return true
		}

		handle, ok := methods.X.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := handle.Fun.(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "Handle" && selector.Sel.Name != "HandleFunc") {
			return true
		}

		router, ok := selector.X.(*ast.Ident)
		if !ok || router.Name != "api" || len(handle.Args) == 0 {
			return true
		}

		path, ok := stringLiteral(handle.Args[0])
		if !ok {
			return true
		}

		for _, arg := range call.Args {
			method, ok := stringLiteral(arg)
			if ok && method != "OPTIONS" {
				routes[method+" "+path] = true
			}
		}

		return true
	})

	return routes
}

func addRouteLiteral(routes map[string]bool, literal *ast.CompositeLit) {
	if len(literal.Elts) != 3 {
		return
	}

	path, ok := stringLiteral(literal.Elts[0])
	if !ok || !strings.HasPrefix(path, "/") {
		return
	}

	methods, ok := literal.Elts[2].(*ast.CompositeLit)
	if !ok {
		return
	}

	for _, element := range methods.Elts {
		if method, ok := stringLiteral(element); ok {
			routes[method+" "+path] = true
		}
	}
}

func routeSetFromText(t *testing.T, routes string) map[string]bool {
	t.Helper()

	result := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(routes))

	for scanner.Scan() {
		if route := strings.TrimSpace(scanner.Text()); route != "" {
			result[route] = true
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scan expected routes: %v", err)
	}

	return result
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(literal.Value)

	return value, err == nil
}
