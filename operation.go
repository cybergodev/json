package json

// ============================================================================
// INTERNAL OPERATION TYPES
// These types are used internally for tracking operation types during processing.
// ============================================================================

// operation represents the type of operation being performed
type operation int

const (
	opGet operation = iota
	opSet
	opDelete
)

// HookContext.Operation / log operation-name strings (single source of truth).
const (
	opNameGet    = "get"
	opNameSet    = "set"
	opNameDelete = "delete"
)

// String returns the string representation of the operation
func (op operation) String() string {
	switch op {
	case opGet:
		return opNameGet
	case opSet:
		return opNameSet
	case opDelete:
		return opNameDelete
	default:
		return "unknown"
	}
}
