// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gitlab.com/fisherprime/hierarchy/v3/lexer"
)

// Deserialization errors.
var (
	ErrExcessiveValues     = errors.New("the deserialization source has excessive values")
	ErrExcessiveEndMarkers = fmt.Errorf("the deserialization source has excessive end markers")
)

// Deserialize transforms a serialized tree into a Hierarchy.
//
// An invalid entry will result in a truncated Hierarchy.
func Deserialize[T Constraint](ctx context.Context, l *lexer.Lexer, options ...Option[T]) (h *Hierarchy[T], err error) {
	go l.Lex(ctx)

	var v T
	h = New(v, options...)

	if _, err = h.deserialize(ctx, l); err != nil {
		err = fmt.Errorf("%w: %w", ErrInvalidHierarchySrc, err)
		return
	}

	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	default:
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
// Using JSON to deserialize the input to the intended type.
func (h *Hierarchy[T]) deserialize(ctx context.Context, l *lexer.Lexer, options ...Option[T]) (end bool, err error) {
	var rootValue T

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

	// Alternative to decoding runes.
	var value T
	if err = json.Unmarshal(item.Val, &value); err != nil {
		return
	}

	select {
	case <-ctx.Done():
		end = true
		err = ctx.Err()

		return
	default:
		h.value = value

		for {
			var endChildren bool

			// NOTE: Receivers are passed by copy & need to be initialized; a pointer to nil won't
			// store the results.
			child := New(rootValue, options...)
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
