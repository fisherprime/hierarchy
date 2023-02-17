// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"errors"
	"fmt"

	"github.com/davecgh/go-spew/spew"
)

type (
	// Builder defines an interface for entities that can be read into a Hierarchy.
	Builder interface {
		// Value obtains the value stored by the Builder.
		Value() string
		// Parent obtains the parent stored by the Builder
		Parent() string
	}

	// BuilderList is a wrapper type for []Builder.
	BuilderList []Builder

	// DefaultBuilder is a sample Builder interface implementation.
	DefaultBuilder struct {
		value  string
		parent string
	}
)

// Hierarchy building errors.
var (
	ErrBuildHierarchy = errors.New("failed to build hierarchy")

	ErrMissingRootNode   = errors.New("missing root node")
	ErrMultipleRootNodes = errors.New("hierarchy has multiple root nodes")

	ErrEmptyHierarchySrc      = errors.New("empty hierarchy source")
	ErrInvalidHierarchySrc    = errors.New("invalid hierarchy source")
	ErrInconsistentBuildCache = errors.New("inconsistency between hierarchy and build cache")

	ErrLocateParents = errors.New("unable to locate parents(s)")

	ErrPanicked = errors.New("recovery from panic")
)

// Value obtains the value stored by the DefaultBuilder.
func (d *DefaultBuilder) Value() string { return d.value }

// Parent obtains the parent stored by the DefaultBuilder
func (d *DefaultBuilder) Parent() string { return d.parent }

// Cut a value at some index from the BuilderList.
func (b *BuilderList) Cut(index int) {
	upper := index + 1

	// index == 0.
	if index < 1 {
		// length of slice == 0.
		if upper >= len(*b) {
			*b = BuilderList{}
			return
		}

		*b = (*b)[1:]

		return
	}

	(*b) = append((*b)[:index], (*b)[upper:]...)
}

// NewHierarchy generates a Hierarchy from a BuilderList.
func (b *BuilderList) NewHierarchy(ctx context.Context) (h *Hierarchy, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %v", ErrBuildHierarchy, err)
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrPanicked, r)
		}

		if err != nil {
			fLogger.Debugf("current hierarchy: %s \nsource remnants: %s", spew.Sprint(h), spew.Sprint(b))
			err = fmt.Errorf("%w: %v", ErrInvalidHierarchySrc, err)
		}
	}()

	cache := make(map[string]struct{})
	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	default:
		if len(*b) < 1 {
			err = ErrEmptyHierarchySrc
			return
		}

		rootIndex := 0
		for index := range *b {
			if (*b)[index].Parent() != "" {
				continue
			}

			// Disallow additional root node(s).
			if h != nil {
				err = ErrMultipleRootNodes
				return
			}
			id := (*b)[index].Value()
			h, cache[id] = New(id), struct{}{}

			rootIndex = index
		}
		if h == nil {
			err = ErrMissingRootNode
			return
		}

		// Remove the root node.
		prevLen := len(*b)
		fLogger.Debugf("BuilderList: %+v\n", *b)
		b.Cut(rootIndex)
		fLogger.Debugf("BuilderList (less root): %+v\n", *b)

		for {
			lenSrc := len(*b)
			if lenSrc < 1 {
				return
			}

			if lenSrc == prevLen {
				err = fmt.Errorf("%w for: %s", ErrLocateParents, spew.Sprint(b))
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

				var parent *Hierarchy
				if parent, err = h.Locate(ctx, parentID); err != nil {
					if errors.Is(err, ErrNotFound) {
						// Inconsistency between the cache & hierarchy.
						err = fmt.Errorf("%w: %v", ErrInconsistentBuildCache, err)
					}

					return
				}

				childID := node.Value()
				if err = parent.AddChild(ctx, New(childID)); err != nil {
					return
				}
				cache[childID] = struct{}{}

				b.Cut(index)

				// Allow for unordered BuilderLists.
				//
				// Adds extraneous opcodes compared to the ordered BuilderList's operations
				// commented below.
				break

				/* index--
				 * lenSrc-- */
			}
		}
	}
}
