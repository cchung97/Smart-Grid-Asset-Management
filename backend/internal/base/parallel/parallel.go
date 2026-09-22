// Package parallel fans an index range out across goroutines: a chunked
// worker pool whose workers write to disjoint parts of a caller-owned
// slice, so results stay in input order with no locking. It knows nothing
// about what the work is.
package parallel

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
)

// defaultChunkSize is how many indices one worker takes at a time — large
// enough that channel overhead is noise, small enough to balance load.
const defaultChunkSize = 512

// Chunks calls fn(lo, hi) over [0, n) split into chunks, on up to
// GOMAXPROCS goroutines. fn must only write to indices in [lo, hi) of any
// shared slice, which keeps results in input order with no locking. It
// runs inline when the input is too small to be worth fanning out.
//
// A panic in fn is returned as an error, and a cancelled ctx stops the
// remaining chunks and is returned as ctx.Err().
func Chunks(ctx context.Context, n int, fn func(lo, hi int)) error {
	return chunksN(ctx, n, defaultChunkSize, runtime.GOMAXPROCS(0), fn)
}

func chunksN(ctx context.Context, n, chunk, workers int, fn func(lo, hi int)) error {
	if n <= 0 {
		return ctx.Err()
	}
	chunks := (n + chunk - 1) / chunk
	workers = min(workers, chunks)
	if workers < 2 {
		if err := runChunk(fn, 0, n); err != nil {
			return err
		}
		return ctx.Err()
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		jobs     = make(chan [2]int)
	)
	for w := 0; w < workers; w++ {
		wg.Go(func() {
			// Keep draining even after a failure so the producer never blocks.
			for job := range jobs {
				if ctx.Err() != nil {
					continue
				}
				if err := runChunk(fn, job[0], job[1]); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
			}
		})
	}
	for lo := 0; lo < n; lo += chunk {
		if ctx.Err() != nil {
			break
		}
		jobs <- [2]int{lo, min(lo+chunk, n)}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

// runChunk converts a panic in a worker into an error: an unrecovered
// panic on a non-request goroutine would take the whole process down.
func runChunk(fn func(lo, hi int), lo, hi int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in chunk [%d, %d): %v\n%s", lo, hi, r, debug.Stack())
		}
	}()
	fn(lo, hi)
	return nil
}
