// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"sort"
	"strings"

	"gitlab.com/fisherprime/hierarchy/lexer"
)

// Serialize transforms a Hierarchy into a string.
func (h *Hierarchy) Serialize(ctx context.Context, opts lexer.Opts) (output string, err error) {
	select {
	case <-ctx.Done():
		return
	default:
		(&opts).Validate()

		serChan := make(chan string)
		go func() {
			h.serialize(ctx, opts, serChan)
			close(serChan)
		}()

		// Handle root Hierarchy.
		fVal, fProceed := <-serChan
		if !fProceed {
			return
		}
		var buffer strings.Builder
		if _, err = buffer.WriteString(fVal); err != nil {
			// Invalidate serialization output.
			return
		}

		for {
			val, proceed := <-serChan
			if !proceed {
				break
			}

			if val != string(opts.EndMarker) {
				if _, err = buffer.WriteString(string(opts.Splitter)); err != nil {
					return
				}
			}
			if _, err = buffer.WriteString(val); err != nil {
				// Invalidate serialization output.
				return
			}
		}

		output = buffer.String()
	}

	return
}

// serialize performs the serialization grunt work.
func (h *Hierarchy) serialize(ctx context.Context, opts lexer.Opts, serChan chan string) {
	if h == nil || h.value == "" {
		return
	}
	serChan <- h.value

	// Create a sorted slice to hold the child *Hierarchy(ies) for serialization.
	//
	// Using the map directly is not guaranteed to follow the same order yielding valid but
	// different serialization output.
	index, lenChildren := 0, len(h.children)
	sortedChildren := make(List, lenChildren)
	for _, child := range h.children {
		sortedChildren[index] = child
		index++
	}
	sort.Sort(&sortedChildren)

	for _, child := range sortedChildren {
		select {
		case <-ctx.Done():
			return
		default:
			child.serialize(ctx, opts, serChan)
		}
	}
	serChan <- string(opts.EndMarker)
}
