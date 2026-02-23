package json

import (
	"strings"

	"github.com/cybergodev/json/internal"
)

func (p *Processor) splitPath(path string, segments []PathSegment) []PathSegment {
	segments = segments[:0]

	if !p.needsPathPreprocessing(path) {
		return p.splitPathIntoSegments(path, segments)
	}

	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	processedPath := p.preprocessPath(path, sb)

	return p.splitPathIntoSegments(processedPath, segments)
}

func (p *Processor) needsPathPreprocessing(path string) bool {
	return internal.NeedsPathPreprocessing(path)
}

func (p *Processor) preprocessPath(path string, sb *strings.Builder) string {
	return internal.PreprocessPath(path, sb)
}

func (p *Processor) needsDotBefore(prevChar rune) bool {
	return internal.NeedsDotBefore(prevChar)
}

func (p *Processor) splitPathIntoSegments(path string, segments []PathSegment) []PathSegment {
	return internal.SplitPathIntoSegments(path, segments)
}

func (p *Processor) parsePathSegment(part string, segments []PathSegment) []PathSegment {
	return internal.ParsePathSegment(part, segments)
}

func (p *Processor) parsePath(path string) ([]string, error) {
	if path == "" {
		return []string{}, nil
	}

	if !p.isComplexPath(path) {
		return strings.Split(path, "."), nil
	}

	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	result := make([]string, len(segments))
	for i, segment := range segments {
		result[i] = segment.String()
	}

	return result, nil
}

func (p *Processor) isDistributedOperationPath(path string) bool {
	return internal.IsDistributedOperationPath(path)
}

func (p *Processor) isDistributedOperationSegment(segment PathSegment) bool {
	return internal.IsDistributedOperationSegment(segment)
}

func (p *Processor) handleDistributedOperation(data any, segments []PathSegment) (any, error) {
	return p.getValueWithDistributedOperation(data, p.reconstructPath(segments))
}

func (p *Processor) reconstructPath(segments []PathSegment) string {
	return internal.ReconstructPath(segments)
}

// parseArraySegment parses array access segments like [0], [1:3], etc.
func (p *Processor) parseArraySegment(part string, segments []PathSegment) []PathSegment {
	return internal.ParseArraySegment(part, segments)
}

// parseExtractionSegment parses extraction segments like {key}, {flat:key}, etc.
func (p *Processor) parseExtractionSegment(part string, segments []PathSegment) []PathSegment {
	return internal.ParseExtractionSegment(part, segments)
}
