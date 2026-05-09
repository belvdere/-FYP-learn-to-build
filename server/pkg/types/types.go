package types

// Symbol represents a code symbol (class, method, constructor, etc.).
type Symbol struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"` // "class", "method", "constructor", "field", "interface"
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`
	Col       int    `json:"col,omitempty"` // 0-indexed column of the name identifier
	EndLine   int    `json:"endLine,omitempty"`
	Signature string `json:"signature,omitempty"`
	BodyHash  string `json:"bodyHash,omitempty"`
}

// MethodCall represents a method invocation relationship.
type MethodCall struct {
	ID       int    `json:"id"`
	CallerID string `json:"callerSymbol"` // Qualified name of calling method (JSON tag kept for wire compatibility)
	CalleeID string `json:"calleeSymbol"` // Name of called method (JSON tag kept for wire compatibility)
	CallType string `json:"callType"`     // "direct", "static", "super", "constructor"
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`
}

// ScannedSymbol is a symbol emitted by index.scan for LSP-based call resolution.
type ScannedSymbol struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	FilePath  string `json:"filePath"`
	URI       string `json:"uri"`
	Line      int    `json:"line"` // 1-indexed
	EndLine   int    `json:"endLine"`
	Col       int    `json:"col"` // 0-indexed
	Signature string `json:"signature,omitempty"`
	BodyHash  string `json:"bodyHash,omitempty"`
}

// SymbolLocation identifies a source location for symbol resolution.
type SymbolLocation struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`
}

// ResolvedLocation maps a source location to a canonical symbol ID.
type ResolvedLocation struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`
	SymbolID string `json:"symbolId,omitempty"`
}

// VirtualNode represents a user-created node not tied to actual code.
type VirtualNode struct {
	ID               string `json:"id"`
	Label            string `json:"label"`
	Type             string `json:"type"` // "method", "class", "concept", "module"
	VirtualClassID   string `json:"virtualClassId,omitempty"`
	VirtualDirectory string `json:"virtualDirectory,omitempty"`
	IsVirtual        bool   `json:"isVirtual"` // Always true
	Materialized     bool   `json:"materialized"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

// NodeAnnotation represents user annotations for a node (real or virtual).
type NodeAnnotation struct {
	NodeID           string `json:"nodeId"`
	Description      string `json:"description,omitempty"`
	AIRemarks        string `json:"aiRemarks,omitempty"`
	CodeSnippet      string `json:"codeSnippet,omitempty"`
	CustomProperties string `json:"customProperties,omitempty"` // JSON string
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

// EdgeAnnotation represents user annotations for an edge.
type EdgeAnnotation struct {
	EdgeID       string `json:"edgeId"`
	SourceNodeID string `json:"sourceNodeId"`
	TargetNodeID string `json:"targetNodeId"`
	Remarks      string `json:"remarks,omitempty"`
	CallCount    int    `json:"callCount"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// VirtualDirectory represents a user-created directory container.
type VirtualDirectory struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	ParentPath string `json:"parentPath,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
}

// GraphNode represents a unified node (real or virtual) for graph display.
type GraphNode struct {
	ID               string          `json:"id"`
	Label            string          `json:"label"`
	Type             string          `json:"type"` // "method", "class", "concept"
	FilePath         string          `json:"filePath,omitempty"`
	Line             int             `json:"line,omitempty"`
	IsVirtual        bool            `json:"isVirtual"`
	Materialized     bool            `json:"materialized,omitempty"`
	ParentID         string          `json:"parentId,omitempty"`
	VirtualClassID   string          `json:"virtualClassId,omitempty"`
	VirtualDirectory string          `json:"virtualDirectory,omitempty"`
	Annotation       *NodeAnnotation `json:"annotation,omitempty"`
}

// EdgeInfo represents edge information for prompt building.
type EdgeInfo struct {
	SourceID   string          `json:"sourceId"`
	TargetID   string          `json:"targetId"`
	Annotation *EdgeAnnotation `json:"annotation,omitempty"`
}

// SnapshotNode represents a node captured in a snapshot.
type SnapshotNode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	FilePath    string `json:"filePath,omitempty"`
	Line        int    `json:"line,omitempty"`
	IsVirtual   bool   `json:"isVirtual"`
	ParentID    string `json:"parentId,omitempty"`
	Description string `json:"description,omitempty"`
	AIRemarks   string `json:"aiRemarks,omitempty"`
}

// SnapshotEdge represents an edge captured in a snapshot.
type SnapshotEdge struct {
	ID       string `json:"id"`
	SourceID string `json:"sourceId"`
	TargetID string `json:"targetId"`
	Remarks  string `json:"remarks,omitempty"`
}

// Snapshot represents a captured graph state for agentic code generation.
type Snapshot struct {
	ID           string         `json:"id"`
	Name         string         `json:"name,omitempty"`
	VirtualNodes []SnapshotNode `json:"virtualNodes"`
	ContextNodes []SnapshotNode `json:"contextNodes"`
	Edges        []SnapshotEdge `json:"edges"`
	CustomPrompt string         `json:"customPrompt,omitempty"`
	CreatedAt    int64          `json:"createdAt"`
	UpdatedAt    int64          `json:"updatedAt"`
}
