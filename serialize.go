// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"strings"
)

// Serialize transforms a `Node` into a string.
func (h *Node) Serialize(ctx context.Context) (output string, err error) {
	select {
	case <-ctx.Done():
		return
	default:
		serChan := make(chan string)
		go func() {
			h.serialize(ctx, serChan)
			close(serChan)
		}()

		// Handle root `Node`.
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

			if val != string(endMarker) {
				if _, err = buffer.WriteString(string(valueSplitter)); err != nil {
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
func (h *Node) serialize(ctx context.Context, serChan chan string) {
	if h == nil || h.value == h.rootValue {
		return
	}
	serChan <- h.value

	for _, child := range h.children {
		select {
		case <-ctx.Done():
			return
		default:
			child.serialize(ctx, serChan)
		}
	}
	serChan <- string(endMarker)
}
