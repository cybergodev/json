package json

import (
	"strings"
	"sync/atomic"
)

// getPathSegments gets a path segments slice from the unified resource manager
func (p *Processor) getPathSegments() []PathSegment {
	return getGlobalResourceManager().GetPathSegments()
}

// putPathSegments returns a path segments slice to the unified resource manager
func (p *Processor) putPathSegments(segments []PathSegment) {
	getGlobalResourceManager().PutPathSegments(segments)
}

// getStringBuilder gets a string builder from the unified resource manager
func (p *Processor) getStringBuilder() *strings.Builder {
	return getGlobalResourceManager().GetStringBuilder()
}

// putStringBuilder returns a string builder to the unified resource manager
func (p *Processor) putStringBuilder(sb *strings.Builder) {
	getGlobalResourceManager().PutStringBuilder(sb)
}

// performMaintenance performs periodic maintenance tasks
func (p *Processor) performMaintenance() {
	if p.isClosing() {
		return // Skip maintenance if closing
	}

	// Clean expired cache entries
	if p.cache != nil {
		p.cache.CleanExpiredCache()
	}

	// Perform unified resource manager maintenance
	getGlobalResourceManager().PerformMaintenance()

	// Perform leak detection
	if p.resourceMonitor != nil {
		if issues := p.resourceMonitor.CheckForLeaks(); len(issues) > 0 {
			for _, issue := range issues {
				p.logger.Warn("Resource issue detected", "issue", issue)
			}
		}
	}
}

// isClosing returns true if the processor is in the process of closing
func (p *Processor) isClosing() bool {
	state := atomic.LoadInt32(&p.state)
	return state == 1 || state == 2
}
