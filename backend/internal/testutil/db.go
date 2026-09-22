// Package testutil holds helpers shared by integration tests.
package testutil

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"smart-grid-asset-management/backend/internal/base/postgres"
)

// extensionLockID serialises CREATE EXTENSION across test processes
// (go test runs packages in parallel).
const extensionLockID = 7263049

// NewTestDB returns a *sql.DB bound to a brand-new, isolated schema with
// every db/migrations/*.up.sql applied, and drops the schema when the
// test ends — so tests never see each other's rows, run safely in
// parallel, and never touch the developer's real data.
//
// It skips the test unless TEST_DATABASE_URL points at a reachable
// Postgres, e.g.
//
//	docker compose up -d db
//	TEST_DATABASE_URL='postgres://grid:grid@localhost:5432/grid_assets?sslmode=disable' go test ./...
func NewTestDB(t testing.TB) *gorm.DB {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()

	adminGorm, err := postgres.Open(base)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL is set but Postgres is unreachable: %v", err)
	}
	admin, err := adminGorm.DB()
	if err != nil {
		t.Fatalf("admin sql.DB: %v", err)
	}

	// pg_trgm must live in public: the migrations reference gin_trgm_ops,
	// resolved through search_path=<test schema>,public.
	ensureTrigramExtension(t, admin)

	schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}

	db, err := postgres.Open(withSearchPath(t, base, schema))
	if err != nil {
		admin.Close()
		t.Fatalf("open test connection: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("test sql.DB: %v", err)
	}
	t.Cleanup(func() {
		postgres.Close(db)
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})

	for _, file := range migrationFiles(t) {
		sqlText, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if _, err := sqlDB.ExecContext(ctx, string(sqlText)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(file), err)
		}
	}
	return db
}

func ensureTrigramExtension(t testing.TB, admin *sql.DB) {
	t.Helper()
	ctx := context.Background()
	conn, err := admin.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", extensionLockID); err != nil {
		t.Fatalf("advisory lock: %v", err)
	}
	defer conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", extensionLockID)
	if _, err := conn.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public"); err != nil {
		t.Fatalf("create pg_trgm extension: %v", err)
	}
}

func withSearchPath(t testing.TB, base, schema string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL must be a postgres:// URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	return u.String()
}

func migrationFiles(t testing.TB) []string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "db", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found in %s (err=%v)", dir, err)
	}
	sort.Strings(files)
	return files
}
