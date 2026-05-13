package json

import (
	"strconv"
)

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

// String returns the string representation of the operation
func (op operation) String() string {
	switch op {
	case opGet:
		return "get"
	case opSet:
		return "set"
	case opDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// ============================================================================
// INTERNAL ERROR TYPES
// ============================================================================

// arrayExtensionSignal is an internal control-flow value (implements error for
// upward propagation through call stack). It is NOT a real error — it signals
// that an array needs to be extended before the value can be set.
// Used exclusively within the set-operation path when createPaths is true.
type arrayExtensionSignal struct {
	requiredLength int
	currentLength  int
	start          int
	end            int
	step           int
	value          any
}

func (e *arrayExtensionSignal) Error() string {
	return "array extension needed: current length " + strconv.Itoa(e.currentLength) +
		", required length " + strconv.Itoa(e.requiredLength) + " for slice [" +
		strconv.Itoa(e.start) + ":" + strconv.Itoa(e.end) + "]"
}
