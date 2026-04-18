package query

// StatementType identifies the kind of DSL statement.
type StatementType int

const (
	StmtFindPaths StatementType = iota
	StmtFindBreakpoints
	StmtShowDrift
	StmtFindHighRisk
)

// Statement is the top-level AST node.
type Statement struct {
	Type StatementType

	// StmtFindPaths
	From      *Ref
	To        *Ref
	Where     []Predicate
	OrderBy   string // field name
	OrderDesc bool
	Limit     int

	// StmtFindBreakpoints
	BreakpointsFor string // e.g. "top_paths"

	// StmtShowDrift
	DriftSince string // e.g. "last_snapshot"

	// StmtFindHighRisk
	HighRiskTarget string // e.g. "service_accounts"
}

// Ref is a typed identity reference, e.g. "user:alice" or "privilege:tier0".
type Ref struct {
	Kind  string // "user", "group", "privilege", etc.
	Value string // the name
}

// Predicate is a single WHERE clause filter.
type Predicate struct {
	Field    string
	Operator string // ">" "<" ">=" "<=" "=" "!="
	Value    string // always string; executor coerces
}
