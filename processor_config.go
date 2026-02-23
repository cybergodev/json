package json

import (
	"log/slog"
	"sync/atomic"
)

// GetConfig returns a copy of the processor configuration
func (p *Processor) GetConfig() *Config {
	configCopy := *p.config
	return &configCopy
}

// SetLogger sets a custom structured logger for the processor
func (p *Processor) SetLogger(logger *slog.Logger) {
	if logger != nil {
		p.logger = logger.With("component", "json-processor")
	} else {
		p.logger = slog.Default().With("component", "json-processor")
	}
}

// checkClosed returns an error if the processor is closed or closing
func (p *Processor) checkClosed() error {
	state := atomic.LoadInt32(&p.state)
	if state != 0 {
		if state == 1 {
			return &JsonsError{
				Op:      "check_closed",
				Message: "processor is closing",
				Err:     ErrProcessorClosed,
			}
		}
		return &JsonsError{
			Op:      "check_closed",
			Message: "processor is closed",
			Err:     ErrProcessorClosed,
		}
	}
	return nil
}

// prepareOptions prepares and validates processor options
func (p *Processor) prepareOptions(opts ...*ProcessorOptions) (*ProcessorOptions, error) {
	var options *ProcessorOptions
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	} else {
		options = DefaultOptions()
	}

	// Valid options
	if err := ValidateOptions(options); err != nil {
		return nil, err
	}

	return options, nil
}
