// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gitlab.com/fisherprime/hierarchy/lexer/v2"
)

// Deserialization errors.
var (
	ErrExcessiveValues     = errors.New("the deserialization source has excessive values")
	ErrExcessiveEndMarkers = fmt.Errorf("the deserialization source has excessive end markers")
)

// Deserialize transforms a serialized tree into a Hierarchy.
//
// An invalid entry will result in a truncated Hierarchy.
func Deserialize[T Constraint](ctx context.Context, opts ...lexer.Option) (h *Hierarchy[T], err error) {
	select {
	case <-ctx.Done():
		return
	default:
		l := lexer.New(opts...)
		go l.Lex(ctx)

		var v T
		h = New(v)
		if _, err = h.deserialize(ctx, l); err != nil {
			err = fmt.Errorf("%w: %v", ErrInvalidHierarchySrc, err)
			return
		}

		diff := l.ValueCounter() - l.EndCounter()
		switch {
		case diff > 0:
			// Excessive values.
			err = fmt.Errorf("%w: +%d", ErrExcessiveValues, diff)
		case diff < 0:
			// Excessive end markers.
			err = fmt.Errorf("%w: %s +%d", ErrExcessiveEndMarkers, string(l.EndMarker()), diff*-1)

		default:
			// Valid
		}
		if err != nil {
			return
		}

		children, _ := h.AllChildrenByLevel(ctx)
		l.Logger().Debugf("hierarchy: %+v", children)
	}

	return
}

// deserialize performs the deserialization grunt work.
//
// Using json to deserialize the input to the intended type.
func (h *Hierarchy[T]) deserialize(ctx context.Context, l *lexer.Lexer) (end bool, err error) {
	var rootValue T

	select {
	case <-ctx.Done():
		end = true
		return
	default:
		item, proceed := l.Item()
		if !proceed {
			end = true
			return
		}

		l.Logger().Debugf("lexed item: %+v", item)

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

		var dest T
		if err = json.Unmarshal(item.Val, &dest); err != nil {
			return
		}

		*h = *New(dest)
		for {
			var endChildren bool

			// NOTE: Receivers are passed by copy & need to be initialized; a pointer to nil won't
			// store the results.
			child := New(rootValue)
			if endChildren, err = child.deserialize(ctx, l); endChildren || err != nil {
				// End of children.
				return
			}

			if child.value == rootValue {
				continue
			}

			child.parent = h
			h.children[child.value] = child
		}
	}
}
