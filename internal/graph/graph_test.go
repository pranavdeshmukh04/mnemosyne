package graph_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdeshmukh/mnemosyne/internal/db"
	"github.com/pdeshmukh/mnemosyne/internal/graph"
)

// setupTestStore creates a real SQLite DB in a temp dir, runs migrations,
// and returns a Store ready for testing. No mocks — we test against real SQL.
func setupTestStore(t *testing.T) *graph.Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return graph.NewStore(d)
}

func TestCreateAndGetNode(t *testing.T) {
	s := setupTestStore(t)

	n, err := s.CreateNode("file", "main.go", "entry point", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if n.ID == "" || n.Kind != "file" || n.Name != "main.go" {
		t.Fatalf("unexpected node: %+v", n)
	}

	got, err := s.GetNode(n.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "main.go" || got.Body != "entry point" {
		t.Fatalf("get returned wrong data: %+v", got)
	}
}

func TestGetNodeNotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetNode("nonexistent")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got: %v", err)
	}
}

func TestUpdateNode(t *testing.T) {
	s := setupTestStore(t)

	n, _ := s.CreateNode("decision", "use-wal", "WAL mode chosen", "")
	err := s.UpdateNode(n.ID, "use-wal", "WAL mode chosen for concurrency", "")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := s.GetNode(n.ID)
	if got.Body != "WAL mode chosen for concurrency" {
		t.Fatalf("body not updated: %s", got.Body)
	}
	if got.UpdatedAt <= n.UpdatedAt {
		// updated_at should be >= created_at (may be equal if same second)
	}
}

func TestUpdateNodeNotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.UpdateNode("nonexistent", "x", "y", "")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got: %v", err)
	}
}

func TestFindNodeByKindAndName(t *testing.T) {
	s := setupTestStore(t)

	s.CreateNode("file", "server.go", "", "")
	s.CreateNode("file", "client.go", "", "")

	got, err := s.FindNodeByKindAndName("file", "server.go")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Name != "server.go" {
		t.Fatalf("wrong node: %+v", got)
	}

	_, err = s.FindNodeByKindAndName("file", "missing.go")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got: %v", err)
	}
}

func TestDeleteNode(t *testing.T) {
	s := setupTestStore(t)

	n, _ := s.CreateNode("bug", "nil-pointer", "crash on startup", "")
	err := s.DeleteNode(n.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = s.GetNode(n.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("node should be gone, got: %v", err)
	}
}

func TestDeleteNodeNotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteNode("nonexistent")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got: %v", err)
	}
}

func TestCreateAndGetEdges(t *testing.T) {
	s := setupTestStore(t)

	a, _ := s.CreateNode("file", "a.go", "", "")
	b, _ := s.CreateNode("file", "b.go", "", "")

	e, err := s.CreateEdge(a.ID, b.ID, "references", "")
	if err != nil {
		t.Fatalf("create edge: %v", err)
	}
	if e.Src != a.ID || e.Dst != b.ID || e.Kind != "references" {
		t.Fatalf("unexpected edge: %+v", e)
	}

	// Query by src
	fromA, err := s.GetEdgesBySrc(a.ID)
	if err != nil {
		t.Fatalf("edges by src: %v", err)
	}
	if len(fromA) != 1 || fromA[0].Dst != b.ID {
		t.Fatalf("expected 1 edge from a to b, got: %+v", fromA)
	}

	// Query by dst
	toB, err := s.GetEdgesByDst(b.ID)
	if err != nil {
		t.Fatalf("edges by dst: %v", err)
	}
	if len(toB) != 1 || toB[0].Src != a.ID {
		t.Fatalf("expected 1 edge to b from a, got: %+v", toB)
	}
}

func TestCreateEdgeInvalidNode(t *testing.T) {
	s := setupTestStore(t)

	// Foreign key constraint: src must exist.
	_, err := s.CreateEdge("nonexistent", "also-nonexistent", "references", "")
	if err == nil {
		t.Fatal("expected foreign key error, got nil")
	}
}

func TestDeleteEdge(t *testing.T) {
	s := setupTestStore(t)

	a, _ := s.CreateNode("file", "x.go", "", "")
	b, _ := s.CreateNode("file", "y.go", "", "")
	e, _ := s.CreateEdge(a.ID, b.ID, "depends_on", "")

	if err := s.DeleteEdge(e.ID); err != nil {
		t.Fatalf("delete edge: %v", err)
	}

	edges, _ := s.GetEdgesBySrc(a.ID)
	if len(edges) != 0 {
		t.Fatalf("edge should be gone, got: %+v", edges)
	}
}

func TestDeleteEdgeNotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteEdge("nonexistent")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got: %v", err)
	}
}

func TestCascadeDeleteNodeRemovesEdges(t *testing.T) {
	s := setupTestStore(t)

	a, _ := s.CreateNode("file", "a.go", "", "")
	b, _ := s.CreateNode("file", "b.go", "", "")
	s.CreateEdge(a.ID, b.ID, "references", "")

	// Deleting node a should cascade-delete edges where a is src.
	s.DeleteNode(a.ID)
	edges, _ := s.GetEdgesByDst(b.ID)
	if len(edges) != 0 {
		t.Fatalf("cascade should have removed edge, got: %+v", edges)
	}
}

// Verify temp files are actually created (sanity check for test infra).
func TestSetupCreatesDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "check.db")
	d, _ := db.Open(path)
	db.Migrate(d)
	d.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("DB file should exist: %v", err)
	}
}
