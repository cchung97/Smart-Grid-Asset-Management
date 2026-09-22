package router_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"

	_ "smart-grid-asset-management/backend/docs" // registers the generated spec, as main.go does
	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/middleware"
	"smart-grid-asset-management/backend/internal/router"
)

// These tests need no database: the router is built with nil handlers,
// which is fine because only route registration and the docs are exercised.

const docsKey = "test-key"

func docsEngine(t *testing.T) *gin.Engine {
	t.Helper()
	logs.SetOutput(io.Discard)
	t.Cleanup(func() { logs.SetOutput(os.Stdout) })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return router.New(router.Deps{
		APIKey:          docsKey,
		RateLimiter:     middleware.NewRateLimiter(ctx, 1000, 0),
		MaxRequestBytes: 1 << 20,
	})
}

type spec struct {
	Swagger             string                                `json:"swagger"`
	BasePath            string                                `json:"basePath"`
	Paths               map[string]map[string]json.RawMessage `json:"paths"`
	Definitions         map[string]json.RawMessage            `json:"definitions"`
	SecurityDefinitions map[string]json.RawMessage            `json:"securityDefinitions"`
}

func loadSpec(t *testing.T) (spec, string) {
	t.Helper()
	raw, err := swag.ReadDoc()
	if err != nil {
		t.Fatalf("swag.ReadDoc: %v (is backend/docs generated? run `make docs`)", err)
	}
	var s spec
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("the generated spec is not valid JSON: %v", err)
	}
	return s, raw
}

// A route registered in router.New but missing from the spec (or the other
// way round) means the generated docs — and so the frontend types — are stale.
func TestSwagger_DocumentsExactlyTheRegisteredAPIRoutes(t *testing.T) {
	s, _ := loadSpec(t)

	inSpec := map[string]bool{}
	for path, methods := range s.Paths {
		for method := range methods {
			inSpec[strings.ToUpper(method)+" "+path] = true
		}
	}

	param := regexp.MustCompile(`:(\w+)`)
	inRouter := map[string]bool{}
	for _, r := range docsEngine(t).Routes() {
		if r.Path == "/healthz" || strings.HasPrefix(r.Path, "/api/") {
			inRouter[r.Method+" "+param.ReplaceAllString(r.Path, "{$1}")] = true
		}
	}

	if len(inRouter) == 0 {
		t.Fatal("no API routes found on the engine")
	}
	for route := range inRouter {
		if !inSpec[route] {
			t.Errorf("route %q is registered but not in the Swagger spec — run `make docs`", route)
		}
	}
	for route := range inSpec {
		if !inRouter[route] {
			t.Errorf("spec documents %q but no such route is registered", route)
		}
	}
}

func TestSwagger_SpecIsSelfConsistent(t *testing.T) {
	s, raw := loadSpec(t)

	if s.Swagger != "2.0" || s.BasePath != "/" {
		t.Errorf("swagger = %q basePath = %q, want 2.0 and / (paths are written in full)", s.Swagger, s.BasePath)
	}
	if _, ok := s.SecurityDefinitions["ApiKeyAuth"]; !ok {
		t.Error("securityDefinitions.ApiKeyAuth missing")
	}

	// Every $ref must resolve, so a renamed or removed dto fails here rather than
	// leaving the frontend generator with a dangling reference.
	var missing []string
	for _, m := range regexp.MustCompile(`"\$ref":\s*"#/definitions/([^"]+)"`).FindAllStringSubmatch(raw, -1) {
		if _, ok := s.Definitions[m[1]]; !ok {
			missing = append(missing, m[1])
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("unresolved $refs: %v", missing)
	}
	if len(s.Definitions) == 0 {
		t.Error("spec has no definitions")
	}
}

func TestSwagger_EveryAPIOperationDeclaresTheKey(t *testing.T) {
	s, _ := loadSpec(t)
	for path, ops := range s.Paths {
		for method, raw := range ops {
			secured := strings.Contains(string(raw), `"ApiKeyAuth"`)
			switch {
			case strings.HasPrefix(path, "/api/") && !secured:
				t.Errorf("%s %s is under /api but does not declare ApiKeyAuth", strings.ToUpper(method), path)
			case !strings.HasPrefix(path, "/api/") && secured:
				t.Errorf("%s %s is open but declares ApiKeyAuth", strings.ToUpper(method), path)
			}
		}
	}
}

func TestSwagger_UIAndSpecAreServedWithoutAKey(t *testing.T) {
	e := docsEngine(t)

	get := func(target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec
	}

	rec := get("/swagger/doc.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("doc.json = %d, want 200 with no key", rec.Code)
	}
	var served spec
	if err := json.Unmarshal(rec.Body.Bytes(), &served); err != nil || len(served.Paths) == 0 {
		t.Fatalf("served doc.json is not the spec: %v (%d paths)", err, len(served.Paths))
	}
	if want, _ := loadSpec(t); len(served.Paths) != len(want.Paths) {
		t.Errorf("served spec has %d paths, generated spec has %d", len(served.Paths), len(want.Paths))
	}

	rec = get("/swagger/index.html")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "swagger-ui") {
		t.Errorf("index.html = %d, want the Swagger UI page", rec.Code)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "'unsafe-inline'") {
		t.Errorf("Swagger UI page CSP = %q, want the relaxed docs policy (inline init script)", csp)
	}

	if rec := get("/docs"); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/swagger/index.html" {
		t.Errorf("/docs = %d -> %q, want a redirect to /swagger/index.html", rec.Code, rec.Header().Get("Location"))
	}
}
