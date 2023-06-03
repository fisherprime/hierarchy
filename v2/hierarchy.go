// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/exp/constraints"
)

// REF: https://www.geeksforgeeks.org/generic-tree-level-order-traversal
//
// REF: https://www.geeksforgeeks.org/serialize-deserialize-n-ary-tree
//
// runes are singe UTF-8 codepoints, not used as the hierarchy expects strings.

// Constraint is a wrapper interface containing comparable & constraints.Ordered.
type Constraint interface {
	comparable
	constraints.Ordered
}

type (
	// Hierarchy defines an n-array tree to hold hierarchies.
	//
	// Synchronization is unnecessary, the type is designed for single write multiple read.
	Hierarchy[T Constraint] struct {
		// cfg contains a pointer to a [Config] shared by all Hierarchy nodes.
		cfg *Config

		// parent contains a reference to the upper Hierarchy.
		parent *Hierarchy[T]

		// value contains the node's data.
		value T

		// children holds references to nodes at a lower level.
		children children[T]

		// locateCache holds references to located nodes at a lower level.
		locateCache children[T]
	}

	// Config defines configuration options for the [BuildSource] & [Hierarchy]'s operations.
	Config struct {
		// Logger for [Hierarchy] messages.
		//
		// Preferring a public field to allow for sharing.
		Logger logrus.FieldLogger
		Debug  bool
	}

	children[T Constraint] map[T]*Hierarchy[T]

	// List is a type wrapper for []*Hierarchy.
	List[T Constraint]      []*Hierarchy[T]
	LevelList[T Constraint] []List[T]

	// TraverseComm defines a channel to communicate info between [Hierarchy] operations & it's callers.
	TraverseComm[T Constraint] struct {
		node     *Hierarchy[T]
		err      error
		newPeers bool
	}

	// Option defines the Hierarchy functional option type.
	Option[T Constraint] func(*Hierarchy[T])
)

const (
	traverseBufferSize = 10
	// poolSize           = 100

	notChildErrFmt = "(%v) %w (%v)"
)

// Errors encountered when handling a Hierarchy.
var (
	ErrNotFound = errors.New("not found")

	ErrNoLeaves     = errors.New("lacks leaves; tree is cyclic")
	ErrAlreadyChild = errors.New("is a child of")
	ErrNoChildren   = errors.New("lacks children ")
	ErrNotChild     = errors.New("is not a child of")
)

var defConfig = DefConfig()

// DefConfig obtains the package's [Hierarchy] default options.
func DefConfig() *Config {
	return &Config{
		Logger: logrus.New(),
		Debug:  false,
	}
}

// New instantiates a [Hierarchy].
func New[T Constraint](value T, options ...Option[T]) *Hierarchy[T] {
	h := &Hierarchy[T]{
		cfg:      defConfig,
		value:    value,
		children: make(children[T]),
	}

	for _, opt := range options {
		opt(h)
	}

	return h
}

// WithConfig configures the [Hierarchy] [Config].
func WithConfig[T Constraint](cfg *Config) Option[T] {
	return func(h *Hierarchy[T]) { h.cfg = cfg }
}

// UseLocateCache enables the usage of a cache for [hierarchy.Locate] operations.
func UseLocateCache[T Constraint]() Option[T] {
	return func(h *Hierarchy[T]) { h.locateCache = make(children[T]) }
}

// Config retrieves the [Hierarchy]'s Opts.
func (h *Hierarchy[T]) Config() *Config { return h.cfg }

// Value retrieves the [Hierarchy]'s data.
func (h *Hierarchy[T]) Value() T { return h.value }

// Parent retrieves a reference to the [Hierarchy]'s parent.
//
// Value is nil for the root node.
func (h *Hierarchy[T]) Parent() *Hierarchy[T] { return h.parent }

// SetParent for a [Hierarchy].
func (h *Hierarchy[T]) SetParent(parent *Hierarchy[T]) { h.parent = parent }

// Child retrieves an immediate child.
func (h *Hierarchy[T]) Child(_ context.Context, childValue T) (child *Hierarchy[T], ok bool) {
	child, ok = h.children[childValue]
	return
}

// AddChild to a [Hierarchy].
//
// Throws an error on existing child.
func (h *Hierarchy[T]) AddChild(ctx context.Context, child *Hierarchy[T], parent ...T) (err error) {
	parentNode := h

	if len(parent) > 0 {
		if parentNode, err = h.locateParent(ctx, parent[0]); err != nil {
			return
		}
	}

	if _, ok := parentNode.Child(ctx, child.value); ok {
		return fmt.Errorf("(%v) %w (%v)", child.value, ErrAlreadyChild, h.value)
	}

	child.parent = parentNode
	h.children[child.value] = child

	return
}

// PopChild removes an immediate child of the [Hierarchy], returning it's reference.
func (h *Hierarchy[T]) PopChild(ctx context.Context, childValue T, parentValue ...T) (child *Hierarchy[T], err error) {
	parent := h

	if len(parentValue) > 0 {
		if parent, err = h.locateParent(ctx, parentValue[0]); err != nil {
			return
		}
	}

	child, ok := parent.Child(ctx, childValue)
	if !ok {
		err = fmt.Errorf("child (%v) of (%v): %w", childValue, parent.value, ErrNotFound)
		return
	}

	delete(parent.children, childValue)
	parent.clearLocateCacheEntry(ctx, childValue)

	return
}

// Children lists the immediate children for a [Hierarchy].
func (h *Hierarchy[T]) Children(ctx context.Context) (children List[T]) {
	children = make(List[T], len(h.children))

	index := 0
	for key := range h.children {
		children[index] = h.children[key]
		index++
	}

	return
}

// AllChildren lists immediate and children-of children for a [Hierarchy].
func (h *Hierarchy[T]) AllChildren(ctx context.Context, parentValue ...T) (children List[T], err error) {
	parent := h

	if len(parentValue) > 0 {
		if parent, err = h.locateParent(ctx, parentValue[0]); err != nil {
			return
		}
	}

	children = make(List[T], 0)
	traverseChan := make(chan TraverseComm[T], traverseBufferSize)

	go parent.Walk(ctx, traverseChan)

	for {
		resl, proceed := <-traverseChan
		if !proceed {
			break
		}
		if resl.err != nil {
			err = resl.err
			return
		}

		children = append(children, resl.node)
	}

	if parent.cfg.Debug {
		parent.cfg.Logger.Debugf("walked: %+v", children)
	}

	if len(children) > 0 {
		// Omit self from the list.
		children = (children)[1:]

		if parent.cfg.Debug {
			parent.cfg.Logger.Debugf("children: %+v", children)
		}
	}

	if len(children) < 1 {
		err = ErrNoChildren
	}

	return
}

// AllChildrenByLevel lists immediate and children-of children for a [Hierarchy] by level.
func (h *Hierarchy[T]) AllChildrenByLevel(ctx context.Context, parentValue ...T) (children LevelList[T], err error) {
	parent := h

	if len(parentValue) > 0 {
		if parent, err = h.locateParent(ctx, parentValue[0]); err != nil {
			return
		}
	}

	children = make(LevelList[T], 0)
	traverseChan := make(chan TraverseComm[T], traverseBufferSize)

	go parent.Walk(ctx, traverseChan)

	var peers List[T]
	for {
		resl, proceed := <-traverseChan
		if !proceed {
			break
		}
		if err = resl.err; err != nil {
			return
		}

		if !resl.newPeers {
			peers = append(peers, resl.node)
			continue
		}

		if len(peers) > 0 {
			children = append(children, peers)
		}
		peers = List[T]{resl.node}
	}

	if len(peers) > 0 {
		children = append(children, peers)
	}

	if parent.cfg.Debug {
		parent.cfg.Logger.Debugf("walked: %+v", children)
	}

	if len(children) > 0 {
		// Omit self from the list.
		children = (children)[1:]

		if parent.cfg.Debug {
			parent.cfg.Logger.Debugf("children: %+v", children)
		}
	}

	if len(children) < 1 {
		err = ErrNoChildren
	}

	return
}

// Leaves returns an array of terminal [Hierarchy](ies).
//
// An error here indicates a cyclic [Hierarchy].
func (h *Hierarchy[T]) Leaves(ctx context.Context) (termNodes List[T], err error) {
	termNodes = make(List[T], 0)
	traverseChan := make(chan TraverseComm[T], traverseBufferSize)

	go h.Walk(ctx, traverseChan)

	for {
		resl, proceed := <-traverseChan
		if !proceed {
			break
		}
		if err = resl.err; err != nil {
			return
		}

		if len(resl.node.children) < 1 {
			termNodes = append(termNodes, resl.node)
		}
	}

	if len(termNodes) < 1 {
		err = ErrNoLeaves
	}

	return
}

// Locate searches for a value & returns it's [Hierarchy].
func (h *Hierarchy[T]) Locate(ctx context.Context, childValue T, parentValue ...T) (child *Hierarchy[T], err error) {
	parent := h

	if len(parentValue) > 0 {
		if parent, err = h.locateParent(ctx, parentValue[0]); err != nil {
			return
		}
	}

	if parent.value == childValue {
		return parent, nil
	}

	traverseChan := make(chan TraverseComm[T])
	wg := new(sync.WaitGroup)
	wg.Add(1)

	locateCtx, locateCancel := context.WithCancel(ctx)
	defer locateCancel()

	go parent.locate(locateCtx, childValue, traverseChan, wg)
	go func() {
		wg.Wait()
		close(traverseChan)
	}()

	resl, proceed := <-traverseChan
	if !proceed {
		err = fmt.Errorf("(%v) %w", childValue, ErrNotFound)
		return
	}
	locateCancel()

	if child, err = resl.node, resl.err; child != nil {
		if parent.locateCache != nil {
			parent.locateCache[child.value] = child
		}

		return
	}

	err = fmt.Errorf("(%v) %w", childValue, ErrNotFound)

	return
}

func (h *Hierarchy[T]) locate(ctx context.Context, value T, traverseChan chan TraverseComm[T], wg *sync.WaitGroup) {
	defer wg.Done()
	if h.cfg.Debug {
		h.cfg.Logger.Debugf("locate val %s in %+v", value, h)
	}

	if h.value == value {
		traverseChan <- TraverseComm[T]{node: h}
		return
	}

	if len(h.children) < 1 {
		return
	}

	if h.locateCache != nil {
		if node, ok := h.locateCache[value]; ok {
			traverseChan <- TraverseComm[T]{node: node}
			return
		}
	}

	if child, ok := h.Child(ctx, value); ok {
		traverseChan <- TraverseComm[T]{node: child}
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
		internalWG := new(sync.WaitGroup)
		internalWG.Add(len(h.children))
		for _, v := range h.children {
			go v.locate(ctx, value, traverseChan, internalWG)
		}
		internalWG.Wait()
	}
}

func (h *Hierarchy[T]) locateParent(ctx context.Context, parent T) (node *Hierarchy[T], err error) {
	if node, err = h.Locate(ctx, parent); err != nil {
		err = fmt.Errorf("parent %w", err)
	}

	return
}

// Walk performs breadth-first traversal on a [Hierarchy], pushing its values to its channel
// argument.
//
// This operation uses channels to minimize resource wastage.
// A context.Context is used to terminate the walk operation.
func (h *Hierarchy[T]) Walk(ctx context.Context, traverseChan chan TraverseComm[T]) {
	defer close(traverseChan)

	// Default operation is to walk.
	if h == nil {
		return
	}

	// Level order traversal.
	queue := List[T]{h}

	select {
	case <-ctx.Done():
		// Received context cancellation.
		traverseChan <- TraverseComm[T]{err: ctx.Err()}
		return
	default:
		// Use a var for front to ensure the outer scope queue is modified.
		var front *Hierarchy[T]

		for {
			queueLen := len(queue)
			if queueLen < 1 {
				break
			}

			// Iterate over the node's children.
			newPeers := true
			for queueLen > 0 {
				// Pop from queue.
				front, queue = queue[0], queue[1:]
				queueLen--

				// Debug: this operation is noisy.
				/* if h.cfg.Debug {
				 *     h.cfg.Logger.Debugf("front: %+v, peers: %+v", front, newPeers)
				 * } */

				// Send node to caller via the channel.
				traverseChan <- TraverseComm[T]{node: front, newPeers: newPeers}
				newPeers = false

				// Add children to the queue.
				if len(front.children) < 1 {
					continue
				}

				for _, v := range front.children {
					queue = append(queue, v)
				}
			}
		}
	}
}

// clearLocateCacheEntry removes values from the current [Hierarchy] ascending to the root node.
func (h *Hierarchy[T]) clearLocateCacheEntry(ctx context.Context, values ...T) {
	if h.locateCache == nil {
		return
	}

	node, cache := h, h.locateCache
	for cache != nil {
		for _, value := range values {
			delete(cache, value)
		}

		if node.parent == nil {
			break
		}

		// Ascend to parent node.
		node = node.parent
		cache = node.locateCache
	}
}

// Len is the number of elements in the collection.
func (l *List[T]) Len() int { return len(*l) }

// Less reports whether the element with index i must sort before the element with index j.
func (l *List[T]) Less(i int, j int) bool { return (*l)[i].value < (*l)[j].value }

// Swap swaps the elements with indexes i and j.
func (l *List[T]) Swap(i int, j int) { (*l)[i], (*l)[j] = (*l)[j], (*l)[i] }

// Values returns an array of values for a [List[T]].
func (l *List[T]) Values(_ context.Context, sortValues ...bool) (values []T) {
	values = make([]T, len(*l))
	for index := range *l {
		values[index] = (*l)[index].value
	}

	if len(sortValues) > 0 && sortValues[0] {
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	}

	return
}

// Values returns an array-of array of values for a [LevelList[T]].
func (l *LevelList[T]) Values(ctx context.Context, sortValues ...bool) (values [][]T) {
	values = make([][]T, len(*l))
	for index := range *l {
		values[index] = (*l)[index].Values(ctx, sortValues...)
	}

	return
}
