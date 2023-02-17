// SPDX-License-Identifier: MIT
package hierarchy

import (
	"context"
	"errors"
	"fmt"
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
	}

	// Config defines configuration options for the [BuildSource] & [Hierarchy]'s operations.
	Config struct {
		Logger logrus.FieldLogger
		Debug  bool
	}

	children[T Constraint] map[T]*Hierarchy[T]

	// List is a type wrapper for []*Hierarchy.
	List[T Constraint] []*Hierarchy[T]

	// TraverseComm defines a channel to communicate info between Hierarchy operations & it's callers.
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

// DefConfig obtains the package's  Hierarchy default options.
func DefConfig() *Config {
	return &Config{
		Logger: logrus.New(),
		Debug:  false,
	}
}

// New instantiates a Hierarchy node.
func New[T Constraint](value T, options ...Option[T]) *Hierarchy[T] {
	h := &Hierarchy[T]{
		cfg:      DefConfig(),
		value:    value,
		children: make(children[T]),
	}

	for _, opt := range options {
		opt(h)
	}

	return h
}

// WithConfig configures the Hierarchy [Config].
func WithConfig[T Constraint](cfg *Config) Option[T] {
	return func(h *Hierarchy[T]) { h.cfg = cfg }
}

// Config retrieves the Hierarchy's Opts.
func (h *Hierarchy[T]) Config() *Config { return h.cfg }

// Value retrieves the Hierarchy's data.
func (h *Hierarchy[T]) Value() T { return h.value }

// Parent retrieves a reference to the Hierarchy's parent.
//
// Value is nil for the root node.
func (h *Hierarchy[T]) Parent() *Hierarchy[T] { return h.parent }

// SetParent for a Hierarchy.
func (h *Hierarchy[T]) SetParent(parent *Hierarchy[T]) { h.parent = parent }

// HasChild checks for the existence of an immediate child.
func (h *Hierarchy[T]) HasChild(_ context.Context, childID T) (ok bool) {
	_, ok = h.children[childID]
	return
}

// AddChild to a Hierarchy.
//
// Throws an error on existing child.
func (h *Hierarchy[T]) AddChild(ctx context.Context, child *Hierarchy[T]) (err error) {
	if h.HasChild(ctx, child.value) {
		return fmt.Errorf("(%v) %w (%v)", child.value, ErrAlreadyChild, h.value)
	}

	child.parent = h
	h.children[child.value] = child

	return
}

// AddChildTo to a Hierarchy.
func (h *Hierarchy[T]) AddChildTo(ctx context.Context, parentID T, child *Hierarchy[T]) (err error) {
	var parent *Hierarchy[T]
	if parent, err = h.Locate(ctx, parentID); err != nil {
		return fmt.Errorf("parent (%v) %w", parentID, err)
	}

	return parent.AddChild(ctx, child)
}

// PopImmediateChild removes an immediate child to the Hierarchy, returning it's reference.
func (h *Hierarchy[T]) PopChild(ctx context.Context, childID T) (child *Hierarchy[T], err error) {
	if !h.HasChild(ctx, childID) {
		err = fmt.Errorf("child (%v) of (%v): %w", childID, h.value, ErrNotFound)
		return
	}

	child = h.children[childID]
	delete(h.children, childID)

	return
}

// Children lists the immediate children for a Hierarchy.
func (h *Hierarchy[T]) Children(ctx context.Context) (children []T) {
	children = make([]T, len(h.children))

	index := 0
	for key := range h.children {
		children[index] = h.children[key].value
		index++
	}

	return
}

// AllChildren lists immediate and children-of children for a Hierarchy.
//
// NOTE: This operation is expensive.
func (h *Hierarchy[T]) AllChildren(ctx context.Context) (children []T, err error) {
	children = make([]T, 0)
	hierChan := make(chan TraverseComm[T], traverseBufferSize)

	go h.Walk(ctx, hierChan)

	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if resl.err != nil {
			err = resl.err
			return
		}

		children = append(children, resl.node.value)
	}

	h.cfg.Logger.Debugf("Hierarchy walk: %+v", children)

	if len(children) > 0 {
		// Omit self from the list.
		children = (children)[1:]

		h.cfg.Logger.Debugf("children: %+v", children)
	}

	if len(children) < 1 {
		err = ErrNoChildren
	}

	return
}

// AllChildrenByLevel lists immediate and children-of children for a Hierarchy by level.
//
// NOTE: This operation is expensive.
func (h *Hierarchy[T]) AllChildrenByLevel(ctx context.Context) (children [][]T, err error) {
	children = make([][]T, 0)
	hierChan := make(chan TraverseComm[T], traverseBufferSize)

	go h.Walk(ctx, hierChan)

	var peers []T
	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if err = resl.err; err != nil {
			return
		}

		if !resl.newPeers {
			peers = append(peers, resl.node.value)
			continue
		}

		if len(peers) > 0 {
			children = append(children, peers)
		}
		peers = []T{resl.node.value}
	}

	if len(peers) > 0 {
		children = append(children, peers)
	}

	h.cfg.Logger.Debugf("Hierarchy walk: %+v", children)

	if len(children) > 0 {
		// Omit self from the list.
		children = (children)[1:]
		h.cfg.Logger.Debugf("children: %+v", children)
	}

	if len(children) < 1 {
		err = ErrNoChildren
	}

	return
}

// AllChildrenOf performs the AllChildren operation for some parent in the Hierarchy.
func (h *Hierarchy[T]) AllChildrenOf(ctx context.Context, parentID T) (children []T, err error) {
	node, err := h.Locate(ctx, parentID)
	if err != nil {
		return
	}

	return node.AllChildren(ctx)
}

// AllChildrenOfByLevel performs the AllChildren operation for some parent in the Hierarchy by level.
func (h *Hierarchy[T]) AllChildrenOfByLevel(ctx context.Context, parentID T) (children [][]T, err error) {
	node, err := h.Locate(ctx, parentID)
	if err != nil {
		return
	}

	return node.AllChildrenByLevel(ctx)
}

// LeafNodes returns an array of terminal Hierarchy(ies).
func (h *Hierarchy[T]) LeafNodes(ctx context.Context) (termNodes List[T], err error) {
	termNodes = make(List[T], 0)
	hierChan := make(chan TraverseComm[T], traverseBufferSize)

	go h.Walk(ctx, hierChan)

	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if err = resl.err; err != nil {
			return
		}

		if resl.node.children == nil || len(resl.node.children) < 1 {
			termNodes = append(termNodes, resl.node)
		}
	}

	if len(termNodes) < 1 {
		err = ErrNoLeaves
	}

	return
}

// Leaves returns an array of leaves (terminal node) values as defined in Hierarchy.
func (h *Hierarchy[T]) Leaves(ctx context.Context) (termValues []T, err error) {
	nodes, err := h.LeafNodes(ctx)
	if err != nil {
		return
	}

	termValues = make([]T, len(nodes))
	for index := range nodes {
		termValues[index] = nodes[index].value
	}

	return
}

// ParentTo returns the parent Hierarchy for some node identified by its id.
func (h *Hierarchy[T]) ParentTo(ctx context.Context, childID T) (parent *Hierarchy[T], err error) {
	var node *Hierarchy[T]
	if node, err = h.Locate(ctx, childID); err != nil {
		err = fmt.Errorf("(%v) %w", childID, err)
		return
	}

	// Update the variable to hold the parent node.
	parent = node.parent

	return
}

// Locate searches for an id & returns it's Hierarchy.
func (h *Hierarchy[T]) Locate(ctx context.Context, id T) (node *Hierarchy[T], err error) {
	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	default:
		if h.Value() == id {
			return h, nil
		}

		hierChan := make(chan TraverseComm[T])
		wg := new(sync.WaitGroup)
		wg.Add(1)
		go h.locate(ctx, id, hierChan, wg)
		go func() {
			wg.Wait()
			close(hierChan)
		}()

		resl, proceed := <-hierChan
		if !proceed {
			err = ErrNotFound
			return
		}

		if node, err = resl.node, resl.err; node != nil {
			return
		}

		err = ErrNotFound
	}

	return
}

func (h *Hierarchy[T]) locate(ctx context.Context, id T, hierChan chan TraverseComm[T], wg *sync.WaitGroup) {
	defer wg.Done()
	// h.cfg.Logger.Debugf("locate val %s in %+v", id, h)

	select {
	case <-ctx.Done():
		return
	default:
		if h.value == id {
			hierChan <- TraverseComm[T]{node: h}
			return
		}

		if len(h.children) < 1 {
			return
		}

		if node, ok := h.children[id]; ok {
			hierChan <- TraverseComm[T]{node: node}
			return
		}

		internalWG := new(sync.WaitGroup)
		internalWG.Add(len(h.children))
		for _, v := range h.children {
			go v.locate(ctx, id, hierChan, internalWG)
		}
		internalWG.Wait()
	}
}

// ChildTo searches for the child to some parent & returns it's Hierarchy.
func (h *Hierarchy[T]) ChildTo(ctx context.Context, parentID, childID T) (child *Hierarchy[T], err error) {
	var parent *Hierarchy[T]
	if parent, err = h.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf("parent (%v) %w", parentID, err)
		return
	}

	if child, err = parent.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf(notChildErrFmt, childID, ErrNotChild, parentID)
	}

	return
}

// Walk performs level-order traversal on a Hierarchy, pushing its values to its channel
// argument.
//
// This operation uses channels to minimize resource wastage.
// A context.Context is used to terminate the walk operation.
func (h *Hierarchy[T]) Walk(ctx context.Context, hierChan chan TraverseComm[T]) {
	defer close(hierChan)

	select {
	case <-ctx.Done():
		// Received context cancelation.
		return
	default:
		// Default operation is to walk.
		if h == nil {
			return
		}

		// Level order traversal.
		queue := List[T]{h}

		for {
			qLen := len(queue)
			if qLen < 1 {
				break
			}

			// Iterate over the node's children.
			newPeers := true
			for qLen > 0 {
				// Pop from queue.
				var front *Hierarchy[T]
				front, queue = queue[0], queue[1:]
				qLen--

				// Debug: this operation is noisy.
				// h.cfg.Logger.Debugf("front: %+v, peers: %+v", front, newPeers)

				// Send node to caller via the channel.
				hierChan <- TraverseComm[T]{node: front, newPeers: newPeers}
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

// Len is the number of elements in the collection.
func (l *List[T]) Len() int { return len(*l) }

// Less reports whether the element with index i must sort before the element with index j.
func (l *List[T]) Less(i int, j int) bool { return (*l)[i].value < (*l)[j].value }

// Swap swaps the elements with indexes i and j.
func (l *List[T]) Swap(i int, j int) { (*l)[i], (*l)[j] = (*l)[j], (*l)[i] }
