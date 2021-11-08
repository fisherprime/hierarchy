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
		GetHierarchySuperior() string
		GetName() string
		IsAdminUnitHierarchy() bool
	}
)

const (
	sameSourceCounterLimit = 2
)

// Hierarchy building errors.
var (
	ErrEmptyHierarchyBuildSrc = errors.New("empty hierarchy build source")
	ErrInvalidHierarchySrc    = errors.New("invalid hierarchy source")
	ErrLocateSuperiors        = errors.New("unable to locate superior(s)")
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

// FromBuilder generates a hierarchy from a list of items & superiors.
func FromBuilder(ctx context.Context, src []Builder) (h *Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v: %w", r, ErrOpPanic)
		}

		if err != nil {
			lLogger.Debugf("current hierarchy: %s \nsource remnants", spew.Sprint(h), spew.Sprint(src))
			err = fmt.Errorf("%v: %w", ErrInvalidHierarchySrc, err)
		}
	}()

	select {
	case <-ctx.Done():
		err = fmt.Errorf("build hierarchy: %w", ctx.Err())
		return
	default:
		if len(src) < 1 {
			err = ErrEmptyHierarchyBuildSrc
			return
		}

		// Work around for the administrative unit dataset's usage of "0".
		rootIndex := 0
		if src[rootIndex].IsAdminUnitHierarchy() {
			for index := range src {
				if src[index].GetHierarchyID() == RootID {
					// Disallow additional root node.
					if h != nil {
						err = fmt.Errorf("administrative unit %w", ErrMultipleRootNodes)
						return
					}
					h = New(ctx, src[index].GetHierarchyID())
					rootIndex = index

					continue
				}
			}
		} else {
			for index := range src {
				if src[index].GetHierarchySuperior() == RootID {
					// Disallow role id 0 as superior.
					if h != nil {
						err = fmt.Errorf("role %w", ErrMultipleRootNodes)
						return
					}
					h = New(ctx, src[index].GetHierarchyID())
					rootIndex = index

					continue
				}
			}
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
				err = ErrLocateSuperiors
				return
			}
			prevLen = lenSrc

			for index := 0; index < lenSrc; index++ {
				node := src[index]

				var superiorNode *Node
				if superiorNode, err = h.Locate(ctx, node.GetHierarchySuperior()); err != nil {
					if errors.Is(err, ErrIDNotFound) {
						continue
					}
					err = fmt.Errorf("build hierarchy: %w", err)

					return
				}

				if err = superiorNode.AddSubordinate(ctx, New(ctx, node.GetHierarchyID())); err != nil {
					return
				}
				src = popHierarchyBuilder(src, index)

				// NOTE: This works around improperly ordered record identifiers; adds extraneous
				// opcodes.
				break

				/* index--
				 * lenSrc-- */
			}
		}
	}
}
