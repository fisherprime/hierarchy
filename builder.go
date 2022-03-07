// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"errors"
	"fmt"

	"github.com/davecgh/go-spew/spew"
)

type (
	// Builder defines an interface for entities that can be read into a `HierarchyNode`.
	Builder interface {
		// Value obtains the value stored by the `Builder`.
		Value() string
		// Parent obtains the parent stored by the `Builder`
		Parent() string
	}

	// BuilderList is a wrapper type for `[]Builder`.
	BuilderList []Builder

	// DefaultBuilder is a sample `Builder` interface implementation.
	DefaultBuilder struct {
		value  string
		parent string
	}
)

// Hierarchy building errors.
var (
	ErrMissingRootNode        = errors.New("missing root node")
	ErrEmptyHierarchyBuildSrc = errors.New("empty hierarchy build source")
	ErrInvalidBuildCache      = errors.New("invalid hierarchy build cache")
	ErrInvalidHierarchySrc    = errors.New("invalid hierarchy source")
	ErrLocateParents          = errors.New("unable to locate parents(s)")
	ErrMultipleRootNodes      = errors.New("hierarchy has multiple root nodes")

	ErrPanicked = errors.New("operation panic'ed")
)

// Value obtains the value stored by the `DefaultBuilder`.
func (d *DefaultBuilder) Value() string { return d.value }

// Parent obtains the parent stored by the `DefaultBuilder`
func (d *DefaultBuilder) Parent() string { return d.parent }

// Pop a value at some index from the `BuilderList`.
func (b *BuilderList) Pop(index int) {
	base := index - 1
	upper := index + 1

	// index == 0.
	if base < 0 {
		// length of slice == 0.
		if upper >= len(*b) {
			*b = BuilderList{}
			return
		}

		*b = (*b)[1:]

		return
	}

	(*b) = append((*b)[:base], (*b)[upper:]...)
}

func (b *BuilderList) NewHierarchy(ctx context.Context, rootValue ...string) (h *Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrPanicked, r)
		}

		if err != nil {
			fLogger.Debugf("current hierarchy: %s \nsource remnants: %s", spew.Sprint(h), spew.Sprint(b))
			err = fmt.Errorf("%w: %v", ErrInvalidHierarchySrc, err)
		}
	}()

	rVal := DefaultRootValue
	if len(rootValue) == 1 {
		rVal = rootValue[0]
	}

	cache := make(map[string]struct{})
	select {
	case <-ctx.Done():
		err = fmt.Errorf("build hierarchy: %w", ctx.Err())
		return
	default:
		if len(*b) < 1 {
			err = ErrEmptyHierarchyBuildSrc
			return
		}

		rootIndex := 0
		for index := range *b {
			if (*b)[index].Parent() != rVal {
				continue
			}

			// Disallow additional root node(s).
			if h != nil {
				err = ErrMultipleRootNodes
				return
			}
			id := (*b)[index].Value()
			h, cache[id] = New(ctx, id, rVal), struct{}{}

			rootIndex = index
		}
		if h == nil {
			err = ErrMissingRootNode
			return
		}

		// Remove the root node.
		prevLen := len(*b)
		fLogger.Debugf("BuilderList: %+v\n", *b)
		b.Pop(rootIndex)
		fLogger.Debugf("BuilderList (less root): %+v\n", *b)

		for {
			lenSrc := len(*b)
			if lenSrc < 1 {
				return
			}

			if lenSrc == prevLen {
				err = ErrLocateParents
				return
			}
			prevLen = lenSrc

			for index := 0; index < lenSrc; index++ {
				node := (*b)[index]
				parentID := node.Parent()

				// Parent not in hierarchy.
				if _, ok := cache[parentID]; !ok {
					continue
				}

				var parent *Node
				if parent, err = h.Locate(ctx, parentID); err != nil {
					if errors.Is(err, ErrNotFound) {
						// Inconsistency between the cache & hierarchy.
						err = fmt.Errorf("%w: %v", ErrInvalidBuildCache, err)
					}
					err = fmt.Errorf("build hierarchy: %w", err)

					return
				}

				childID := node.Value()
				if err = parent.AddChild(ctx, New(ctx, childID, rVal)); err != nil {
					return
				}
				cache[childID] = struct{}{}

				b.Pop(index)

				// Allow for unordered `BuilderList`s.
				//
				// Adds extraneous opcodes compared to the ordered `BuilderList`'s operations
				// commented below.
				break

				/* index--
				 * lenSrc-- */
			}
		}
	}
}
