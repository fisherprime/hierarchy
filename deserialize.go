// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/fisherprime/hierarchy/lexer"
)

// Deserialization errors.
var (
	ErrEmptyDeserializationSrc = errors.New("empty deserialization source")
	ErrExcessiveValues         = errors.New("the deserialization source has values in excess by")
	ErrExcessiveEndMarkers     = fmt.Errorf("the deserialization source has end marker `%s` in excess by", string(lexer.EndMarker))
)

// Deserialize transforms a serialized tree into a `Node`.
//
// An invalid entry will result in a truncated `Node`.
func Deserialize(ctx context.Context, input string) (h *Node, err error) {
	if input == "" {
		err = ErrEmptyDeserializationSrc
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
		l := lexer.New(fLogger, input)
		go l.Lex(ctx)

		h = New(ctx, DefaultRootValue)
		if _, err = h.deserialize(ctx, l); err != nil {
			err = fmt.Errorf("%w: %v", ErrInvalidHierarchySrc, err)
			return
		}

		diff := l.ValueCounter - l.EndCounter
		switch {
		case diff > 0:
			// Excessive values.
			err = fmt.Errorf("%w: %d", ErrExcessiveValues, diff)
		case diff < 0:
			// Excessive end markers.
			err = fmt.Errorf("%w: %d", ErrExcessiveEndMarkers, diff*-1)
		default:
			// Valid
		}
		if err != nil {
			return
		}

		children, _ := h.ListChildrenOfByLevel(ctx, DefaultRootValue)
		fLogger.Debugf("hierarchy: %+v", children)
	}

	return
}

// deserialize performs the deserialization grunt work.
func (h *Node) deserialize(ctx context.Context, l *lexer.Lexer) (end bool, err error) {
	select {
	case <-ctx.Done():
		end = true
		return
	default:
		item, proceed := <-l.C
		if !proceed {
			end = true
			return
		}

		fLogger.Debugf("lexed item: %+v", item)

		switch item.ID {
		case lexer.ItemEOF:
			end = true
			return
		case lexer.ItemError:
			// Stop input processing.
			end = true
			err = item.Err
			return
		case lexer.ItemEndMarker:
			end = true
			return
		case lexer.ItemSplitter:
			return
		}

		h = New(ctx, item.Val, h.rootValue)
		for {
			var endChildren bool

			// NOTE: Receivers are passed by copy & need to be initialized; a pointer to nil won't
			// store the results.
			child := New(ctx, DefaultRootValue, h.rootValue)
			if endChildren, err = child.deserialize(ctx, l); endChildren || err != nil {
				// End of children.
				return
			}
			child.parent = h

			if child.value != h.rootValue {
				h.children[child.value] = child
			}
		}
	}
}
