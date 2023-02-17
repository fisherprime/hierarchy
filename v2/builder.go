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
	Builder[T Constraint] interface {
		// Value obtains the value stored by the Builder.
		Value() T
		// Parent obtains the parent stored by the Builder
		Parent() T
	}

	// BuildSource is a wrapper type for []Builder used to generate the Hierarchy.
	BuildSource[T Constraint] struct {
		cfg Config

		builders  []Builder[T]
		isOrdered bool
	}

	// BuildOption defines the BuildSource functional option type.
	BuildOption[T Constraint] func(*BuildSource[T])
)

// Hierarchy building errors.
var (
	ErrBuildHierarchy = errors.New("failed to build hierarchy")

	ErrMissingRootNode   = errors.New("missing root node")
	ErrMultipleRootNodes = errors.New("hierarchy has multiple root nodes")

	ErrEmptyHierarchySrc      = errors.New("empty hierarchy source")
	ErrInvalidHierarchySrc    = errors.New("invalid hierarchy source")
	ErrInconsistentBuildCache = errors.New("inconsistency between the hierarchy and build cache")

	ErrLocateParents = errors.New("unable to locate parents(s)")

	ErrPanicked = errors.New("recovery from panic")
)

// Value obtains the value stored by the DefaultBuilder.
func (d *DefaultBuilder) Value() string { return d.value }

// Parent obtains the parent stored by the DefaultBuilder
func (d *DefaultBuilder) Parent() string { return d.parent }

// NewBuildSource instantiates a BuildSource.
func NewBuildSource[T Constraint](options ...BuildOption[T]) *BuildSource[T] {
	b := &BuildSource[T]{
		cfg:       *DefConfig(),
		builders:  []Builder[T]{},
		isOrdered: false,
	}

	for _, opt := range options {
		opt(b)
	}

	return b
}

// WithBuildConfig configures the [BuildSource]'s [Config].
func WithBuildConfig[T Constraint](cfg *Config) BuildOption[T] {
	return func(b *BuildSource[T]) { b.cfg = *cfg }
}

// WithBuilders configures the underlying list.
func WithBuilders[T Constraint](builders []Builder[T]) BuildOption[T] {
	return func(b *BuildSource[T]) { b.builders = builders }
}

// Len retrieves the length of the BuildSource.
func (b *BuildSource[T]) Len() int { return len(b.builders) }

// Cut a value at some index from the BuildSource.
func (b *BuildSource[T]) Cut(index int) {
	if index == 0 {
		b.builders = b.builders[1:]
		return
	}

	upper := index + 1
	// Cut upto (excluding) `index`, cut from (including) `index+1`.
	b.builders = append(b.builders[:index], b.builders[upper:]...)
}

// Build generates a hierarchy from a Source.
func (b *BuildSource[T]) Build(ctx context.Context) (h *Hierarchy[T], err error) {
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
			b.cfg.Logger.Debugf("current hierarchy: %s \nsource remnants: %s", spew.Sprint(h), spew.Sprint(b))
			err = fmt.Errorf("%w: %v", ErrInvalidHierarchySrc, err)
		}
	}()

	if b.Len() < 1 {
		err = ErrEmptyHierarchySrc
		return
	}

	var rootValue T
	cache := make(map[T]struct{})

	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	default:
		rootIndex := 0
		for index := range b.builders {
			if b.builders[index].Parent() != rootValue {
				continue
			}

			// Disallow additional root node(s).
			if h != nil {
				err = ErrMultipleRootNodes
				return
			}
			id := b.builders[index].Value()
			h, cache[id] = New(id), struct{}{}

			rootIndex = index
		}
		if h == nil {
			err = ErrMissingRootNode
			return
		}

		// Remove the root node from the build source..
		prevLen := b.Len()
		b.cfg.Logger.Debugf("source: %+v\n", *b)
		b.Cut(rootIndex)
		b.cfg.Logger.Debugf("source (without root): %+v\n", *b)

		for {
			lenSrc := b.Len()
			if lenSrc < 1 {
				return
			}

			if lenSrc == prevLen {
				err = fmt.Errorf("%w for: %s", ErrLocateParents, spew.Sprint(b))
				return
			}
			prevLen = lenSrc

			for index := 0; index < lenSrc; index++ {
				node := b.builders[index]
				parentID := node.Parent()

				// Parent not in hierarchy.
				if _, ok := cache[parentID]; !ok {
					continue
				}

				var parent *Hierarchy[T]
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

				// Remove added node from the build source.
				b.Cut(index)

				// Allow for unordered Sources.
				//
				// Adds extraneous opcodes compared to the ordered Source's operation.
				if !b.isOrdered {
					break
				}

				index--
				lenSrc--
			}
		}
	}
}
