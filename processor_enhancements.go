package json

import (
	"context"
	"time"
)

// ProcessorEnhancements provides enhanced processor functionality
type ProcessorEnhancements struct {
	processor     *Processor
	dosProtection *DOSProtection
	enableDOS     bool
}

// NewEnhancedProcessor creates a processor with enhanced security and monitoring
func NewEnhancedProcessor(config *Config, enableDOS bool) *ProcessorEnhancements {
	processor := New(config)

	var dosProtection *DOSProtection
	if enableDOS {
		dosProtection = NewDOSProtection(DefaultDOSConfig())
	}

	return &ProcessorEnhancements{
		processor:     processor,
		dosProtection: dosProtection,
		enableDOS:     enableDOS,
	}
}

// Get retrieves a value with DOS protection
func (pe *ProcessorEnhancements) Get(jsonStr, path string, opts ...*ProcessorOptions) (any, error) {
	// Check DOS protection
	if pe.enableDOS && pe.dosProtection != nil {
		if err := pe.dosProtection.CheckRequest(int64(len(jsonStr)), ""); err != nil {
			return nil, err
		}
		defer pe.dosProtection.ReleaseRequest()

		// Validate depth
		if err := pe.dosProtection.ValidateDepth(jsonStr); err != nil {
			return nil, err
		}
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add context to options
	var enhancedOpts *ProcessorOptions
	if len(opts) > 0 && opts[0] != nil {
		enhancedOpts = opts[0].Clone()
	} else {
		enhancedOpts = &ProcessorOptions{}
	}
	enhancedOpts.Context = ctx

	return pe.processor.Get(jsonStr, path, enhancedOpts)
}

// Set sets a value with DOS protection
func (pe *ProcessorEnhancements) Set(jsonStr, path string, value any, opts ...*ProcessorOptions) (string, error) {
	// Check DOS protection
	if pe.enableDOS && pe.dosProtection != nil {
		if err := pe.dosProtection.CheckRequest(int64(len(jsonStr)), ""); err != nil {
			return jsonStr, err
		}
		defer pe.dosProtection.ReleaseRequest()

		// Validate depth
		if err := pe.dosProtection.ValidateDepth(jsonStr); err != nil {
			return jsonStr, err
		}
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add context to options
	var enhancedOpts *ProcessorOptions
	if len(opts) > 0 && opts[0] != nil {
		enhancedOpts = opts[0].Clone()
	} else {
		enhancedOpts = &ProcessorOptions{}
	}
	enhancedOpts.Context = ctx

	return pe.processor.Set(jsonStr, path, value, enhancedOpts)
}

// Delete deletes a value with DOS protection
func (pe *ProcessorEnhancements) Delete(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	// Check DOS protection
	if pe.enableDOS && pe.dosProtection != nil {
		if err := pe.dosProtection.CheckRequest(int64(len(jsonStr)), ""); err != nil {
			return jsonStr, err
		}
		defer pe.dosProtection.ReleaseRequest()

		// Validate depth
		if err := pe.dosProtection.ValidateDepth(jsonStr); err != nil {
			return jsonStr, err
		}
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add context to options
	var enhancedOpts *ProcessorOptions
	if len(opts) > 0 && opts[0] != nil {
		enhancedOpts = opts[0].Clone()
	} else {
		enhancedOpts = &ProcessorOptions{}
	}
	enhancedOpts.Context = ctx

	return pe.processor.Delete(jsonStr, path, enhancedOpts)
}

// GetProcessor returns the underlying processor
func (pe *ProcessorEnhancements) GetProcessor() *Processor {
	return pe.processor
}

// GetDOSStats returns DOS protection statistics
func (pe *ProcessorEnhancements) GetDOSStats() DOSStats {
	if pe.dosProtection != nil {
		return pe.dosProtection.GetStats()
	}
	return DOSStats{}
}

// Close closes the enhanced processor
func (pe *ProcessorEnhancements) Close() error {
	if pe.processor != nil {
		return pe.processor.Close()
	}
	return nil
}
