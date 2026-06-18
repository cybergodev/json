package json

import (
	"strings"
	"sync"
	"testing"

	"github.com/cybergodev/json/internal"
)

// TestConcurrentCacheSafety tests concurrent access to various caches and
// shared state. All caches follow the same access pattern, so they are
// consolidated into a single table-driven test.
func TestConcurrentCacheSafety(t *testing.T) {
	cache := internal.GetGlobalCompiledPathCache()

	tests := []struct {
		name        string
		concurrency int
		iterations  int
		workload    func(workerID, iteration int) error
	}{
		{
			name:        "PathTypeCache",
			concurrency: 20,
			iterations:  100,
			workload: func(_, _ int) error {
				for _, path := range []string{"simple", "nested.path", "array[0]", "complex.nested[1].key", "very.deep.nested.path.with.many.segments"} {
					_ = getPathType(path)
				}
				return nil
			},
		},
		{
			name:        "KeyInternMap",
			concurrency: 20,
			iterations:  100,
			workload: func(_, _ int) error {
				for _, key := range []string{"id", "name", "value", "timestamp", "metadata"} {
					internal.GlobalKeyIntern.Intern(key)
				}
				return nil
			},
		},
		{
			name:        "DefaultProcessor",
			concurrency: 20,
			iterations:  1,
			workload: func(_, _ int) error {
				p := getDefaultProcessor()
				if p == nil {
					return errPtr("expected non-nil processor")
				}
				_, err := p.Get(`{"test": 1}`, "test")
				return err
			},
		},
		{
			name:        "PathSegmentCache",
			concurrency: 20,
			iterations:  50,
			workload: func(_, _ int) error {
				for _, path := range []string{"simple", "nested.path", "array[0].item"} {
					if segments, ok := internal.GlobalPathIntern.Get(path); ok {
						if len(segments) == 0 {
							return errPtr("empty segments for path " + path)
						}
					}
				}
				return nil
			},
		},
		{
			name:        "CompiledPathCache",
			concurrency: 20,
			iterations:  50,
			workload: func(_, _ int) error {
				for _, path := range []string{"simple", "nested.path", "array[0]"} {
					cp, err := cache.Get(path)
					if err != nil {
						return err
					}
					if cp == nil {
						return errPtr("nil CompiledPath for " + path)
					}
					cp.Release()
				}
				return nil
			},
		},
		{
			name:        "ValidationCache",
			concurrency: 10,
			iterations:  50,
			workload: func(_, _ int) error {
				p, err := New()
				if err != nil {
					return err
				}
				defer p.Close()
				for _, jsonStr := range []string{`{"test": 1}`, `{"nested": {"key": "value"}}`, `{"array": [1, 2, 3]}`} {
					_, err := p.Get(jsonStr, "test")
					if err != nil && !strings.Contains(err.Error(), "not found") {
						return err
					}
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			errCh := make(chan error, tt.concurrency)

			for i := 0; i < tt.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for j := 0; j < tt.iterations; j++ {
						if err := tt.workload(workerID, j); err != nil {
							select {
							case errCh <- err:
							default:
							}
							return
						}
					}
				}(i)
			}

			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Errorf("concurrent %s failed: %v", tt.name, err)
			}
		})
	}
}

// errPtr is a helper to create an error from a string for use in table-driven tests.
func errPtr(msg string) error { return &testErr{msg: msg} }

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
