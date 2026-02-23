package json

// ExtractionContext represents the context for extraction operations
type ExtractionContext struct {
	OriginalContainers []any  // Original containers that hold the target arrays
	ArrayFieldName     string // Name of the array field being operated on
	TargetIndices      []int  // Target indices for each container
	OperationType      string // Type of operation: "get", "set", "delete"
}

// ArrayExtensionNeededError signals that array extension is needed
type ArrayExtensionNeededError struct {
	RequiredLength int
	CurrentLength  int
	Start          int
	End            int
	Step           int
	Value          any
}

func (e *ArrayExtensionNeededError) Error() string {
	return e.error()
}

func (e *ArrayExtensionNeededError) error() string {
	return "array extension needed: current length " + itoa(e.CurrentLength) +
		", required length " + itoa(e.RequiredLength) + " for slice [" +
		itoa(e.Start) + ":" + itoa(e.End) + "]"
}

// itoa converts int to string without importing strconv (for error message efficiency)
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	negative := i < 0
	if negative {
		i = -i
	}

	var buf [32]byte
	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if negative {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}
