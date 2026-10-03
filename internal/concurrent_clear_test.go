package internal

import (
	"sync"
	"testing"
)

// TestInternClearConcurrent exercises every global interner with Clear() racing
// against concurrent mutating/read access. The library's hot path (Intern/Get/Set)
// is already covered by TestConcurrentCacheSafety in the root package; the gap this
// test fills is the Clear() side, which reassigns internal maps and (for KeyIntern)
// previously reassigned the sync.Map field itself — a DATA RACE with concurrent
// readers. Run under the race detector:
//
//	go test -race -run TestInternClearConcurrent ./internal/
//
// (TSan cannot start on the maintainer's Windows host — error 87 on shadow-memory
// allocation — so this is the repro/verification harness for a Linux/WSL run.)
func TestInternClearConcurrent(t *testing.T) {
	keyAlts := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}
	strAlts := []string{"one", "two", "three", "four", "five", "six", "seven", "eight"}

	run := func(name string, clear, work func()) {
		t.Run(name, func(t *testing.T) {
			const workers = 12
			const iterations = 400
			var wg sync.WaitGroup

			// One goroutine repeatedly clears while others hammer the structure.
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range iterations {
					clear()
				}
			}()

			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for i := 0; i < iterations; i++ {
						work()
					}
				}(w)
			}
			wg.Wait()
		})
	}

	// KeyIntern: Clear() must not race with Intern/InternBytes/GetStats. This is
	// the regression target for the fixed hotKeys field reassignment.
	run("KeyIntern", GlobalKeyIntern.Clear, func() {
		for _, k := range keyAlts {
			_ = GlobalKeyIntern.Intern(k)
			_ = GlobalKeyIntern.InternBytes([]byte(k))
		}
		_ = GlobalKeyIntern.GetStats()
	})

	// StringIntern: Clear() racing with Intern/InternBytes/GetStats.
	run("StringIntern", GlobalStringIntern.Clear, func() {
		for _, s := range strAlts {
			_ = GlobalStringIntern.Intern(s)
			_ = GlobalStringIntern.InternBytes([]byte(s))
		}
		_ = GlobalStringIntern.GetStats()
	})
}
