// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gitlab.com/fisherprime/hierarchy/v3/lexer"
)

// Serialize transforms a Hierarchy into a string.
func (h *Hierarchy[T]) Serialize(ctx context.Context, cfg *lexer.Config) (output string, err error) {
	cfg.Validate()

	serChan := make(chan string)
	go func() {
		h.serialize(ctx, cfg, serChan)
		close(serChan)
	}()

	// Handle root Hierarchy.
	fValue, fProceed := <-serChan
	if !fProceed {
		return
	}
	var buffer strings.Builder
	if _, err = buffer.WriteString(fValue); err != nil {
		// Invalidate serialization output.
		return
	}

	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	default:
		for {
			value, proceed := <-serChan
			if !proceed {
				break
			}

			if value != string(cfg.EndMarker) {
				if _, err = buffer.WriteString(string(cfg.Splitter)); err != nil {
					return
				}
			}
			if _, err = buffer.WriteString(value); err != nil {
				// Invalidate serialization output.
				return
			}
		}

		output = buffer.String()
	}

	return
}

// serialize performs the serialization grunt work.
func (h *Hierarchy[T]) serialize(ctx context.Context, cfg *lexer.Config, serChan chan string) {
	var rootValue T
	if h == nil || h.value == rootValue {
		return
	}
	serChan <- fmt.Sprint(h.value)

	// Create a sorted slice to hold the child [Hierarchy](ies) for serialization.
	//
	// Using the map directly is not guaranteed to follow the same order yielding valid but
	// different serialization output.
	index, lenChildren := 0, len(h.children)
	sortedChildren := make(List[T], lenChildren)
	for _, child := range h.children {
		sortedChildren[index] = child
		index++
	}
	sort.Sort(&sortedChildren)

	select {
	case <-ctx.Done():
		// NOTE: context error captured in [Hierarchy.Serialize].
		return
	default:
		for _, child := range sortedChildren {
			child.serialize(ctx, cfg, serChan)
		}
	}
	serChan <- string(cfg.EndMarker)
}
