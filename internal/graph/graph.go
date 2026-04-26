package graph

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Node represents a single entity or conversation in the graph.
type Node struct {
	ID        string
	Kind      string
	Name      string
	Body      string
	Metadata  string
	CreatedAt int64
	UpdatedAt int64
}

// Store wraps a *sql.DB and provides all graph operations.
// Every method takes plain arguments and returns plain structs —
// no ORM, no magic. SQL stays visible and debuggable.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateNode inserts a new node and returns it with a generated UUID.
// 'kind' is one of: conversation, file, function, decision, bug, concept, person, other.
// 'name' is the display name (for file nodes, use the relative path).
func (s *Store) CreateNode(kind, name, body, metadata string) (*Node, error) {
	now := time.Now().Unix()
	id := uuid.New().String()

	_, err := s.db.Exec(
		`INSERT INTO nodes (id, kind, name, body, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, kind, name, body, metadata, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert node: %w", err)
	}

	return &Node{
		ID: id, Kind: kind, Name: name,
		Body: body, Metadata: metadata,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// GetNode retrieves a single node by ID. Returns sql.ErrNoRows if not found.
func (s *Store) GetNode(id string) (*Node, error) {
	n := &Node{}
	err := s.db.QueryRow(
		`SELECT id, kind, name, body, metadata, created_at, updated_at
		 FROM nodes WHERE id = ?`, id,
	).Scan(&n.ID, &n.Kind, &n.Name, &n.Body, &n.Metadata, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}
	return n, nil
}

// UpdateNode modifies an existing node's name, body, and metadata.
// It bumps updated_at to the current time. Kind and created_at are immutable.
func (s *Store) UpdateNode(id, name, body, metadata string) error {
	now := time.Now().Unix()
	res, err := s.db.Exec(
		`UPDATE nodes SET name = ?, body = ?, metadata = ?, updated_at = ?
		 WHERE id = ?`,
		name, body, metadata, now, id,
	)
	if err != nil {
		return fmt.Errorf("update node %s: %w", id, err)
	}

	// RowsAffected tells us if the UPDATE actually matched a row.
	// If 0, the ID doesn't exist — we surface this explicitly rather
	// than silently succeeding.
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("update node %s: %w", id, sql.ErrNoRows)
	}
	return nil
}

// FindNodeByKindAndName looks up a node by its (kind, name) pair.
// Returns sql.ErrNoRows (wrapped) if not found. This is used to check
// for duplicates before creating — e.g., two "file" nodes with the
// same path should be the same node.
func (s *Store) FindNodeByKindAndName(kind, name string) (*Node, error) {
	n := &Node{}
	err := s.db.QueryRow(
		`SELECT id, kind, name, body, metadata, created_at, updated_at
		 FROM nodes WHERE kind = ? AND name = ?`, kind, name,
	).Scan(&n.ID, &n.Kind, &n.Name, &n.Body, &n.Metadata, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find node (%s, %s): %w", kind, name, err)
	}
	return n, nil
}

// Edge represents a directed, typed relationship between two nodes.
type Edge struct {
	ID        string
	Src       string
	Dst       string
	Kind      string
	Metadata  string
	CreatedAt int64
}

// CreateEdge inserts a directed edge from src to dst.
// 'kind' is one of: touched, decided, caused, fixed, references, depends_on, related_to.
// Both src and dst must be existing node IDs (enforced by foreign keys).
func (s *Store) CreateEdge(src, dst, kind, metadata string) (*Edge, error) {
	now := time.Now().Unix()
	id := uuid.New().String()

	_, err := s.db.Exec(
		`INSERT INTO edges (id, src, dst, kind, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, src, dst, kind, metadata, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert edge: %w", err)
	}

	return &Edge{
		ID: id, Src: src, Dst: dst,
		Kind: kind, Metadata: metadata,
		CreatedAt: now,
	}, nil
}

// GetEdgesBySrc returns all edges originating from a given node.
// For example, all relationships a decision node has outward.
func (s *Store) GetEdgesBySrc(src string) ([]Edge, error) {
	return s.queryEdges(`SELECT id, src, dst, kind, metadata, created_at FROM edges WHERE src = ?`, src)
}

// GetEdgesByDst returns all edges pointing to a given node.
// For example, all conversations that "touched" a file node.
func (s *Store) GetEdgesByDst(dst string) ([]Edge, error) {
	return s.queryEdges(`SELECT id, src, dst, kind, metadata, created_at FROM edges WHERE dst = ?`, dst)
}

// queryEdges is a helper that runs an edge query and scans the results.
// Shared by GetEdgesBySrc and GetEdgesByDst to avoid duplicating scan logic.
func (s *Store) queryEdges(query string, args ...any) ([]Edge, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}
	defer rows.Close()

	var edges []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.ID, &e.Src, &e.Dst, &e.Kind, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// DeleteEdge removes an edge by ID.
func (s *Store) DeleteEdge(id string) error {
	res, err := s.db.Exec(`DELETE FROM edges WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete edge %s: %w", id, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("delete edge %s: %w", id, sql.ErrNoRows)
	}
	return nil
}

// DeleteNode removes a node by ID. Thanks to ON DELETE CASCADE in the
// schema, all edges and transcripts referencing this node are automatically
// deleted by SQLite.
func (s *Store) DeleteNode(id string) error {
	res, err := s.db.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete node %s: %w", id, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("delete node %s: %w", id, sql.ErrNoRows)
	}
	return nil
}
