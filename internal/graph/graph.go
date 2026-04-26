package graph

import (
	"database/sql"
	"errors"
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

// FindOrCreateNode returns the existing node matching (kind, name), or creates
// a new one if none exists. This is an idempotent upsert — calling it twice
// with the same arguments always returns the same node ID.
func (s *Store) FindOrCreateNode(kind, name, body, metadata string) (*Node, bool, error) {
	existing, err := s.FindNodeByKindAndName(kind, name)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("find or create node: %w", err)
	}

	created, err := s.CreateNode(kind, name, body, metadata)
	if err != nil {
		return nil, false, fmt.Errorf("find or create node: %w", err)
	}
	return created, true, nil
}

// Neighbor holds a node reachable from a starting point along with the edge
// that connects them.
type Neighbor struct {
	Node Node
	Edge Edge
}

// Neighbors returns all nodes directly connected to nodeID (both outgoing and
// incoming edges) up to the given depth. depth=1 returns immediate neighbors
// only. depth=2 also returns their neighbors, and so on.
func (s *Store) Neighbors(nodeID string, depth int) ([]Node, []Edge, error) {
	if depth < 1 {
		depth = 1
	}

	visitedNodes := map[string]Node{}
	visitedEdges := map[string]Edge{}
	frontier := []string{nodeID}

	for d := 0; d < depth; d++ {
		var next []string
		for _, id := range frontier {
			outEdges, err := s.GetEdgesBySrc(id)
			if err != nil {
				return nil, nil, fmt.Errorf("neighbors (src %s): %w", id, err)
			}
			inEdges, err := s.GetEdgesByDst(id)
			if err != nil {
				return nil, nil, fmt.Errorf("neighbors (dst %s): %w", id, err)
			}

			for _, e := range append(outEdges, inEdges...) {
				if _, seen := visitedEdges[e.ID]; seen {
					continue
				}
				visitedEdges[e.ID] = e

				neighborID := e.Dst
				if neighborID == id {
					neighborID = e.Src
				}
				if _, seen := visitedNodes[neighborID]; !seen {
					n, err := s.GetNode(neighborID)
					if err != nil {
						return nil, nil, fmt.Errorf("neighbors get node %s: %w", neighborID, err)
					}
					visitedNodes[neighborID] = *n
					next = append(next, neighborID)
				}
			}
		}
		frontier = next
	}

	nodes := make([]Node, 0, len(visitedNodes))
	for _, n := range visitedNodes {
		nodes = append(nodes, n)
	}
	edges := make([]Edge, 0, len(visitedEdges))
	for _, e := range visitedEdges {
		edges = append(edges, e)
	}
	return nodes, edges, nil
}

// Search finds nodes whose name or body contains the query string (case-insensitive).
// Optionally filter by kind (pass "" to search all kinds). Results are capped at limit.
func (s *Store) Search(query, kind string, limit int) ([]Node, error) {
	pattern := "%" + query + "%"

	var (
		rows *sql.Rows
		err  error
	)
	if kind != "" {
		rows, err = s.db.Query(
			`SELECT id, kind, name, body, metadata, created_at, updated_at
			 FROM nodes
			 WHERE kind = ? AND (name LIKE ? OR body LIKE ?)
			 LIMIT ?`,
			kind, pattern, pattern, limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, kind, name, body, metadata, created_at, updated_at
			 FROM nodes
			 WHERE name LIKE ? OR body LIKE ?
			 LIMIT ?`,
			pattern, pattern, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("search nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Kind, &n.Name, &n.Body, &n.Metadata, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// SaveTranscript stores the full text of a conversation. If a transcript for
// this conversation already exists it is replaced.
func (s *Store) SaveTranscript(conversationID, content string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`INSERT INTO transcripts (conversation_id, content, created_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(conversation_id) DO UPDATE SET content = excluded.content`,
		conversationID, content, now,
	)
	if err != nil {
		return fmt.Errorf("save transcript %s: %w", conversationID, err)
	}
	return nil
}

// GetTranscript retrieves the stored transcript for a conversation node.
// Returns sql.ErrNoRows if none has been saved yet.
func (s *Store) GetTranscript(conversationID string) (string, error) {
	var content string
	err := s.db.QueryRow(
		`SELECT content FROM transcripts WHERE conversation_id = ?`, conversationID,
	).Scan(&content)
	if err != nil {
		return "", fmt.Errorf("get transcript %s: %w", conversationID, err)
	}
	return content, nil
}

// SetProjectMeta stores a key-value pair in the project metadata table.
// Calling it again with the same key overwrites the previous value.
func (s *Store) SetProjectMeta(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO project (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set project meta %q: %w", key, err)
	}
	return nil
}

// GetProjectMeta retrieves a value from the project metadata table.
// Returns sql.ErrNoRows if the key doesn't exist.
func (s *Store) GetProjectMeta(key string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM project WHERE key = ?`, key,
	).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get project meta %q: %w", key, err)
	}
	return value, nil
}
