package json

import (
	"fmt"
)

// ProcessBatch processes multiple operations in a single batch
func (p *Processor) ProcessBatch(operations []BatchOperation, opts ...*ProcessorOptions) ([]BatchResult, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	_, err := p.prepareOptions(opts...)
	if err != nil {
		return nil, err
	}

	if len(operations) > p.config.MaxBatchSize {
		return nil, &JsonsError{
			Op:      "process_batch",
			Message: fmt.Sprintf("batch size %d exceeds maximum %d", len(operations), p.config.MaxBatchSize),
			Err:     ErrSizeLimit,
		}
	}

	results := make([]BatchResult, len(operations))

	for i, op := range operations {
		result := BatchResult{ID: op.ID}

		switch op.Type {
		case "get":
			result.Result, result.Error = p.Get(op.JSONStr, op.Path, opts...)
		case "set":
			result.Result, result.Error = p.Set(op.JSONStr, op.Path, op.Value, opts...)
		case "delete":
			result.Result, result.Error = p.Delete(op.JSONStr, op.Path, opts...)
		case "validate":
			result.Result, result.Error = p.Valid(op.JSONStr, opts...)
		default:
			result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
		}

		results[i] = result
	}

	return results, nil
}
