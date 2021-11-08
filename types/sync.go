// SPDX-License-Identifier: NONE
package types

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type (
	// SafeCounter is a thread-safe counter.
	SafeCounter struct {
		m   sync.Mutex
		val int
	}
)

const (
	BufferedErrChanSize = 5
)

// Synchronization errors.
var (
	ErrInvalidGoroutineCount = errors.New("invalid goroutine count")
)

// Inc increments the counter.
func (c *SafeCounter) Inc() {
	c.m.Lock()
	defer c.m.Unlock()
	c.val++
}

// Value returns the current value of the counter.
func (c *SafeCounter) Value() int {
	c.m.Lock()
	defer c.m.Unlock()
	return c.val
}

// MonitorChannels `error`s & completion status.
//
// errPrefix should be in the singular form.
func MonitorChannels(ctx context.Context, operations int, done chan bool, errChan chan error, errPrefix string) (err error) {
	if operations < 1 {
		err = fmt.Errorf("%s %w: %d", errPrefix, ErrInvalidGoroutineCount, operations)
		return
	}

	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
		var routinesTerminated bool
		for index := 0; index < operations; index++ {
			select {
			case _, routinesTerminated = <-done:
				// On channel close.
				routinesTerminated = !routinesTerminated
			case e, proceed := <-errChan:
				if !proceed {
					break
				}

				if err != nil {
					err = fmt.Errorf("%v, %w", err, e)
				} else {
					err = fmt.Errorf("%s %w", errPrefix, e)
				}
			}

			if routinesTerminated {
				break
			}
		}
	}

	return
}
