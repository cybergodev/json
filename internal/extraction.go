package internal

// ExtractionGroup represents a group of consecutive extraction segments
// used for processing complex extraction patterns in JSON paths.
type ExtractionGroup struct {
	Segments []PathSegment
}

// DetectConsecutiveExtractions identifies groups of consecutive extraction segments.
// This is useful for processing complex extraction patterns where multiple
// extractions need to be processed together.
func DetectConsecutiveExtractions(segments []PathSegment) []ExtractionGroup {
	var groups []ExtractionGroup
	var currentGroup []PathSegment

	for _, seg := range segments {
		if seg.Type == ExtractSegment {
			currentGroup = append(currentGroup, seg)
		} else {
			if len(currentGroup) > 0 {
				groups = append(groups, ExtractionGroup{Segments: currentGroup})
				currentGroup = nil
			}
		}
	}

	if len(currentGroup) > 0 {
		groups = append(groups, ExtractionGroup{Segments: currentGroup})
	}

	return groups
}
