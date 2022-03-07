// SPDX-License-Identifier: NONE
package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// REF: https://www.geeksforgeeks.org/generic-tree-level-order-traversal
//
// REF: https://www.geeksforgeeks.org/serialize-deserialize-n-ary-tree
//
// `rune`s are singe `UTF-8` codepoints, not used as the hierarchy expects strings.

type (
	// TraverseChan defines a channel to communicate info between `Node` operations & it's callers.
	TraverseChan struct {
		newPeers bool
		node     *Node
		err      error
	}
)

const (
	// DefaultRootValue is default root node value.
	DefaultRootValue = ""

	initialQueueLen    = 0
	traverseBufferSize = 10

	// endMarker a `rune` indicating the end of a node's children.
	endMarker = ')'

	// valueSplitter is the character used to split the `Node` serialization output.
	valueSplitter = ','

	notChildFmt = "(%s) %w (%s)"
)

// Errors encountered when handling a `Node`.
var (
	ErrNotFound = errors.New("not found")

	ErrNoLeaves     = errors.New("lacks leaves; tree is cyclic")
	ErrAlreadyChild = errors.New("is a child of")
	ErrNoChildren   = errors.New("lacks children ")
	ErrNotChild     = errors.New("is not a child of")
)

var (
	fLogger logrus.FieldLogger = logrus.NewEntry(logrus.New())
)

func SetLogger(l logrus.FieldLogger) { fLogger = l }

// AddChild to a `Node`.
func (h *Node) AddChild(ctx context.Context, child *Node) (err error) {
	if h.children != nil {
		// Search for existing immediate child.
		if h.HasChild(ctx, child.value) > 0 {
			err = fmt.Errorf("(%s) %w (%s)", child.value, ErrAlreadyChild, h.value)
			return
		}
	} else {
		h.children = make(childMap)
	}

	child.parent = h
	h.children[child.value] = child

	return
}

// AddChildTo to a `Node`.
func (h *Node) AddChildTo(ctx context.Context, parentID string, child *Node) (err error) {
	var parent *Node
	if parent, err = h.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf("parent (%s) %w", parentID, err)
		return
	}

	return parent.AddChild(ctx, child)
}

// PopChild removes an immediate child to the `Node`.
func (h *Node) PopChild(ctx context.Context, childID string) (child *Node, err error) {
	var index int
	if index = h.HasChild(ctx, childID); index < 0 {
		err = fmt.Errorf("child (%s) of (%s): %s", childID, h.value, ErrNotFound)
		return
	}

	child = h.children[childID]
	delete(h.children, childID)

	return
}

// HasChild checks for the existence of an immediate child.
func (h *Node) HasChild(_ context.Context, childID string) (index int) {
	index = -1
	if _, ok := h.children[childID]; ok {
		index = 1
	}

	return
}

// ListImmediateChildren lists the immediate children for a `Node`.
func (h *Node) ListImmediateChildren(ctx context.Context) (children []string) {
	children = make([]string, 0)
	for _, child := range h.children {
		children = append(children, child.value)
	}

	return
}

// ListChildrenOf returns a list of children as defined in `Node` for some parent
// model.
//
// NOTE: This operation is expensive.
func (h *Node) ListChildrenOf(ctx context.Context, parentID string) (children []string, err error) {
	children = make([]string, 0)
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.WalkChildrenOf(ctx, parentID, hierChan)

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

	fLogger.Debugf("`Node` walk: %+v", children)

	numChildren := len(children)
	if numChildren > 0 {
		// Omit self from the list.
		children = (children)[1:]

		fLogger.Debugf("children: %+v", children)
	}

	if len(children) < 1 {
		err = ErrNoChildren
	}

	return
}

// ListChildrenOfByLevel returns an array-of-arrays of children as defined in
// `Node` for some parent model.
//
// NOTE: This operation will bottleneck list generation operations.
func (h *Node) ListChildrenOfByLevel(ctx context.Context, parentID string) (children [][]string, err error) {
	children = make([][]string, 0)
	hierChan := make(chan TraverseChan, traverseBufferSize)

	go h.WalkChildrenOf(ctx, parentID, hierChan)

	var peers []string
	for {
		resl, proceed := <-hierChan
		if !proceed {
			break
		}
		if resl.err != nil {
			err = resl.err
			return
		}

		if resl.newPeers {
			if len(peers) > 0 {
				children = append(children, peers)
			}
			peers = make([]string, initialQueueLen)
		}
		peers = append(peers, resl.node.value)
	}
	if len(peers) > 0 {
		children = append(children, peers)
	}

	fLogger.Debugf("`Node` walk: %+v", children)

	numChildren := len(children)
	if numChildren > 0 {
		// Omit self from the list.
		children = (children)[1:]

		fLogger.Debugf("children: %+v", children)
	}

	if len(children) < 1 {
		err = ErrNoChildren
	}

	return
}

// ListLeaves returns an array of leaves (terminal node) data as defined in `Node`.
func (h *Node) ListLeaves(ctx context.Context) (termItems []string, err error) {
	termItems = make([]string, 0)
	hierChan := make(chan TraverseChan, traverseBufferSize)

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

		if resl.node.children == nil || len(resl.node.children) < 1 {
			termItems = append(termItems, resl.node.value)
		}
	}

	if len(termItems) < 1 {
		err = ErrNoLeaves
	}

	return
}

// GetParentTo returns the parent `Node` for some node identified by its `id`.
func (h *Node) GetParentTo(ctx context.Context, childID string) (parent *Node, err error) {
	var node *Node
	if node, err = h.Locate(ctx, childID); err != nil {
		err = fmt.Errorf("(%s) %w", childID, err)
		return
	}

	// Update the variable to hold the parent node.
	parent = node.parent

	return
}

// Locate searches for an `id` & returns it's `Node`.
func (h *Node) Locate(ctx context.Context, id string) (node *Node, err error) {
	select {
	case <-ctx.Done():
		return
	default:
		hierChan := make(chan TraverseChan)
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

func (h *Node) locate(ctx context.Context, id string, hierChan chan TraverseChan, wg *sync.WaitGroup) {
	defer wg.Done()
	// lLogger.Debugf("locate val %s in %+v", id, h)

	select {
	case <-ctx.Done():
		return
	default:
		if h.value == id {
			hierChan <- TraverseChan{node: h}
			return
		}

		if h.children == nil || len(h.children) < 1 {
			return
		}

		if node, ok := h.children[id]; ok {
			hierChan <- TraverseChan{node: node}
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

// LocateChildTo searches for the child to some parent & returns it's `Node`.
func (h *Node) LocateChildTo(ctx context.Context, parentID, childID string) (child *Node, err error) {
	var parent *Node
	if parentID != h.rootValue {
		if parent, err = h.Locate(ctx, parentID); err != nil {
			err = fmt.Errorf("parent (%s) %w", parentID, err)
			return
		}
	} else {
		parent = h
	}

	if child, err = parent.Locate(ctx, parentID); err != nil {
		err = fmt.Errorf(notChildFmt, childID, ErrNotChild, parentID)
	}

	return
}

// WalkChildrenOf traverses a `Node` pushing its values to a channel argument.
//
// Using a channel allowing for processing on a returned value as more are obtained.
func (h *Node) WalkChildrenOf(ctx context.Context, parentID string, hierChan chan TraverseChan) {
	if parentID == h.rootValue {
		h.Walk(ctx, hierChan)
		return
	}

	if node, err := h.Locate(ctx, parentID); err == nil {
		node.Walk(ctx, hierChan)
		return
	}

	close(hierChan)
}

// Walk performs level-order traversal on a `Node`, pushing its values to its channel
// argument.
//
// This operation uses channels to minimize resource wastage.
// A `context.Context` is used to terminate the walk operation.
func (h *Node) Walk(ctx context.Context, hierChan chan TraverseChan) {
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
		queue := make([]*Node, initialQueueLen)
		queue = append(queue, h)

		for {
			qLen := len(queue)
			if qLen < 1 {
				break
			}

			// Iterate over the node's children.
			newPeers := true
			for qLen > 0 {
				// Pop from queue.
				var front *Node
				front, queue = queue[0], queue[1:]
				qLen--

				// Debug: this operation is noisy.
				// lLogger.Debugf("front: %+v, peers: %+v", front, newPeers)

				// Send node to caller via the channel.
				hierChan <- TraverseChan{node: front, newPeers: newPeers}
				newPeers = false

				// Add children to the queue.
				if front.children != nil {
					if front.children == nil {
						continue
					}

					for _, v := range front.children {
						queue = append(queue, v)
					}
				}
			}
		}
	}
}
