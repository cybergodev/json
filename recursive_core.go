package json

import (
	"fmt"

	"github.com/cybergodev/json/internal"
)

// RecursiveProcessor implements true recursive processing for all operations
type RecursiveProcessor struct {
	processor *Processor
}

// NewRecursiveProcessor creates a new unified recursive processor
func NewRecursiveProcessor(p *Processor) *RecursiveProcessor {
	return &RecursiveProcessor{
		processor: p,
	}
}

// ProcessRecursively performs recursive processing for any operation
func (urp *RecursiveProcessor) ProcessRecursively(data any, path string, operation Operation, value any) (any, error) {
	return urp.ProcessRecursivelyWithOptions(data, path, operation, value, false)
}

// ProcessRecursivelyWithOptions performs recursive processing with path creation options
func (urp *RecursiveProcessor) ProcessRecursivelyWithOptions(data any, path string, operation Operation, value any, createPaths bool) (any, error) {
	// Parse path into segments using cached parsing
	segments, err := urp.processor.getCachedPathSegments(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	if len(segments) == 0 {
		switch operation {
		case OpGet:
			return data, nil
		case OpSet:
			return nil, fmt.Errorf("cannot set root value")
		case OpDelete:
			return nil, fmt.Errorf("cannot delete root value")
		}
	}

	// Start recursive processing from root
	result, err := urp.processRecursivelyAtSegmentsWithOptions(data, segments, 0, operation, value, createPaths)
	if err != nil {
		return nil, err
	}

	// Check if any segment in the path was a flat extraction
	// If so, we need special handling to apply flattening and subsequent operations correctly
	if operation == OpGet {
		// Find the LAST flat segment, not the first one
		// This is important for paths like orders{flat:items}{flat:tags}[0:3]
		flatSegmentIndex := -1
		for i, segment := range segments {
			if segment.Type == internal.ExtractSegment && segment.IsFlat {
				flatSegmentIndex = i // Keep updating to find the last one
			}
		}

		if flatSegmentIndex >= 0 {
			// Check if there are any operations after the flat extraction
			hasPostFlatOps := flatSegmentIndex+1 < len(segments)

			if hasPostFlatOps {
				// There are operations after flat extraction - need special handling
				// Process the path in two phases:
				// Phase 1: Process up to and including the flat segment
				// Phase 2: Apply flattening and then process remaining segments

				// Step 1: Process up to and including the flat segment
				preFlatSegments := segments[:flatSegmentIndex+1]
				preFlatResult, err := urp.processRecursivelyAtSegmentsWithOptions(data, preFlatSegments, 0, operation, value, createPaths)
				if err != nil {
					return nil, err
				}

				// Step 2: Apply flattening to the pre-flat result
				var flattened []any
				if resultArray, ok := preFlatResult.([]any); ok {
					urp.deepFlattenResults(resultArray, &flattened)
				} else {
					flattened = []any{preFlatResult}
				}

				// Step 3: Process remaining segments on the flattened result
				postFlatSegments := segments[flatSegmentIndex+1:]
				if len(postFlatSegments) > 0 {
					finalResult, err := urp.processRecursivelyAtSegmentsWithOptions(flattened, postFlatSegments, 0, operation, value, createPaths)
					if err != nil {
						return nil, err
					}
					return finalResult, nil
				}

				return flattened, nil
			}

			// No operations after flat extraction - the flat extraction should have been handled
			// during normal processing, so just return the result as-is
			return result, nil
		}
	}

	return result, nil
}

// processRecursivelyAtSegments recursively processes path segments for any operation
func (urp *RecursiveProcessor) processRecursivelyAtSegments(data any, segments []internal.PathSegment, segmentIndex int, operation Operation, value any) (any, error) {
	return urp.processRecursivelyAtSegmentsWithOptions(data, segments, segmentIndex, operation, value, false)
}

// processRecursivelyAtSegmentsWithOptions recursively processes path segments with path creation options
func (urp *RecursiveProcessor) processRecursivelyAtSegmentsWithOptions(data any, segments []internal.PathSegment, segmentIndex int, operation Operation, value any, createPaths bool) (any, error) {
	// Base case: no more segments to process
	if segmentIndex >= len(segments) {
		switch operation {
		case OpGet:
			return data, nil
		case OpSet:
			return nil, fmt.Errorf("cannot set value: no target segment")
		case OpDelete:
			return nil, fmt.Errorf("cannot delete value: no target segment")
		}
	}

	// Check for extract-then-slice pattern
	if segmentIndex < len(segments)-1 {
		currentSegment := segments[segmentIndex]
		nextSegment := segments[segmentIndex+1]

		// Special handling for {extract}[slice] pattern
		if currentSegment.Type == internal.ExtractSegment && nextSegment.Type == internal.ArraySliceSegment {
			return urp.handleExtractThenSlice(data, currentSegment, nextSegment, segments, segmentIndex, operation, value)
		}
	}

	currentSegment := segments[segmentIndex]
	isLastSegment := segmentIndex == len(segments)-1

	switch currentSegment.Type {
	case internal.PropertySegment:
		return urp.handlePropertySegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ArrayIndexSegment:
		return urp.handleArrayIndexSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ArraySliceSegment:
		return urp.handleArraySliceSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ExtractSegment:
		return urp.handleExtractSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.WildcardSegment:
		return urp.handleWildcardSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	default:
		return nil, fmt.Errorf("unsupported segment type: %v", currentSegment.Type)
	}
}
