// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"errors"
	"fmt"

	"github.com/davecgh/go-spew/spew"
)

// NOTE: Authboss will not be configured for the `load…` functions in this file, do not reference
// `ro.AB`.

type (
	// Builder defines an interface for entities that can be read into a `HierarchyNode`.
	Builder interface {
		GetHierarchyID() string
		GetHierarchyParent() string
		GetName() string
	}
)

const (
	sameSourceCounterLimit = 2
)

// Hierarchy building errors.
var (
	ErrEmptyHierarchyBuildSrc = errors.New("empty hierarchy build source")
	ErrInvalidBuildCache      = errors.New("invalid hierarchy build cache")
	ErrInvalidHierarchySrc    = errors.New("invalid hierarchy source")
	ErrLocateParents          = errors.New("unable to locate parents(s)")
	ErrMultipleRootNodes      = errors.New("hierarchy has multiple root nodes")

	ErrOpPanic = errors.New("operation panicked")
)

// popHierarchyBuilder pops an index from the `[]HierarchyBuilder`
func popHierarchyBuilder(src []Builder, index int) []Builder {
	base := index - 1
	upper := index + 1
	if base < 0 {
		// NOTE: Availed for clarity, using the length before the colon will yield an empty slice.
		if upper >= len(src) {
			return []Builder{}
		}

		return src[1:]
	}

	return append(src[:base], src[index+1:]...)
}

// FromBuilder generates a hierarchy from a list of items & parents.
func FromBuilder(ctx context.Context, src []Builder) (h *Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v: %w", r, ErrOpPanic)
		}

		if err != nil {
			fLogger.Debugf("current hierarchy: %s \nsource remnants: %s", spew.Sprint(h), spew.Sprint(src))
			err = fmt.Errorf("%v: %w", ErrInvalidHierarchySrc, err)
		}
	}()

	cache := make(map[string]struct{})
	select {
	case <-ctx.Done():
		err = fmt.Errorf("build hierarchy: %w", ctx.Err())
		return
	default:
		if len(src) < 1 {
			err = ErrEmptyHierarchyBuildSrc
			return
		}

		rootIndex := 0
		for index := range src {
			if src[index].GetHierarchyParent() != RootID {
				continue
			}

			// Disallow additional root node.
			if h != nil {
				err = ErrMultipleRootNodes
				return
			}
			id := src[index].GetHierarchyID()
			h, cache[id] = New(ctx, id), struct{}{}

			rootIndex = index
		}
		if h == nil {
			err = fmt.Errorf("%w: no root node", ErrInvalidHierarchySrc)
			return
		}

		// Remove the root node.
		prevLen := len(src)
		src = popHierarchyBuilder(src, rootIndex)

		for {
			lenSrc := len(src)
			if lenSrc < 1 {
				return
			}

			if lenSrc == prevLen {
				err = ErrLocateParents
				return
			}
			prevLen = lenSrc

			for index := 0; index < lenSrc; index++ {
				node := src[index]
				parentID := node.GetHierarchyParent()

				// Parent not in hierarchy.
				if _, ok := cache[parentID]; !ok {
					continue
				}

				var parent *Node
				if parent, err = h.Locate(ctx, parentID); err != nil {
					if errors.Is(err, ErrIDNotFound) {
						// Inconsistency between the cache & hierarchy.
						err = fmt.Errorf("%v: %w", ErrInvalidBuildCache, err)
					}
					err = fmt.Errorf("build hierarchy: %w", err)

					return
				}

				childID := node.GetHierarchyID()
				if err = parent.AddChild(ctx, New(ctx, childID)); err != nil {
					return
				}
				cache[childID] = struct{}{}

				src = popHierarchyBuilder(src, index)

				// NOTE: This works around improperly ordered record identifiers; id `1` is a child
				// of id `5`.
				//
				// Adds extraneous opcodes.
				break

				/* index--
				 * lenSrc-- */
			}
		}
	}
}
