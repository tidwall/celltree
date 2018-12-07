// Copyright 2018 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package celltree

const nBits = 7      // 1, 2,  4,   8  - match nNodes with the correct nBits
const nNodes = 128   // 2, 4, 16, 256  - match nNodes with the correct nBits
const maxItems = 128 // max items per node
const minItems = maxItems * 40 / 100
const maxUint64 = uint64(0xFFFFFFFFFFFFFFFF)

type item struct {
	cell uint64
	data interface{}
}

type node struct {
	branch bool   // is a branch (not a leaf)
	items  []item // leaf items
	nodes  []node // child nodes
	count  int    // count of all cells for this node and children
}

// Tree is a uint64 prefix tree
type Tree struct {
	count int   // number of items in tree
	root  *node // root node
}

// Insert inserts an item into the tree. Items are ordered by it's cell.
// The extra param is a simple user context value.
func (tr *Tree) Insert(cell uint64, data interface{}) {
	if tr.root == nil {
		tr.root = new(node)
	}
	tr.root.insert(cell, data, 64-nBits)
	tr.count++
}

// Count returns the number of items in the tree.
func (tr *Tree) Count() int {
	return tr.count
}

func cellIndex(cell uint64, bits uint) int {
	return int(cell >> bits & uint64(nNodes-1))
}

func (n *node) insert(cell uint64, data interface{}, bits uint) {
	if !n.branch {
		// leaf node
		if bits >= nBits && len(n.items) >= maxItems {
			// split leaf. it's at capacity
			n.splitLeaf(bits)
			// insert item again, but this time node is a branch
			n.insert(cell, data, bits)
			// we need to deduct one item from the count, otherwise it'll be
			// the target cell will be counted twice
			n.count--
		} else {
			// find the target index for the new cell
			if len(n.items) == 0 || n.items[len(n.items)-1].cell <= cell {
				// the new cell is greater than the last cell in leaf, so
				// we can just append it
				n.items = append(n.items, item{cell: cell, data: data})
			} else {
				// locate the index of the cell in the leaf
				index := n.findLeafItem(cell)
				// create space for the new cell
				n.items = append(n.items, item{})
				// move other cells over to make room for new cell
				copy(n.items[index+1:], n.items[index:len(n.items)-1])
				// assign the new cell
				n.items[index] = item{cell: cell, data: data}
			}
		}
	} else {
		// branch node
		// locate the index of the child node in the leaf
		index := cellIndex(cell, bits)
		// insert the cell into the child node
		n.nodes[index].insert(cell, data, bits-nBits)
	}
	// increment the node
	n.count++
}

// splitLeaf into a branch
func (n *node) splitLeaf(bits uint) {
	n.branch = true
	n.count = 0
	n.nodes = make([]node, nNodes)
	for i := 0; i < len(n.items); i++ {
		n.insert(n.items[i].cell, n.items[i].data, bits)
	}
	n.items = nil
}

// find an index of the cell using a binary search
func (n *node) findLeafItem(cell uint64) int {
	i, j := 0, len(n.items)
	for i < j {
		h := i + (j-i)/2
		if !(cell < n.items[h].cell) {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}

// Remove removes an item from the tree based on it's cell and data values.
func (tr *Tree) Remove(cell uint64, data interface{}) {
	if tr.root == nil {
		return
	}
	deleted := tr.root.remove(cell, data, 64-nBits, nil)
	if deleted {
		// successfully deleted the item.
		// decrement the count
		tr.count--
	}
}

func (n *node) remove(
	cell uint64, data interface{}, bits uint,
	cond func(data interface{}) bool,
) (deleted bool) {
	if !n.branch {
		// leaf node
		i := n.findLeafItem(cell) - 1
		for ; i >= 0; i-- {
			if n.items[i].cell != cell {
				// did not find
				break
			}
			if (cond == nil && n.items[i].data == data) ||
				(cond != nil && cond(n.items[i].data)) {
				// found the cell, remove it now
				// if the len of items has fallen below 40% of it's cap then
				// shrink the items
				min := cap(n.items) * 40 / 100
				if len(n.items)-1 <= min {
					// shrink and realloc the array
					items := make([]item, len(n.items)-1, cap(n.items)/2)
					copy(items[:i], n.items[:i])
					copy(items[i:], n.items[i+1:len(n.items)])
					n.items = items
				} else {
					// keep the same array
					n.items[i] = item{}
					copy(n.items[i:len(n.items)-1], n.items[i+1:])
					n.items = n.items[:len(n.items)-1]
				}
				deleted = true
				break
			}
		}
	} else {
		// branch node
		index := cellIndex(cell, bits)
		deleted = n.nodes[index].remove(cell, data, bits-nBits, cond)
	}
	if deleted {
		// an item was deleted from this node or a child node
		// decrement the counter
		n.count--
		if n.branch && n.count <= minItems {
			// compact the branch into a leaf
			n.compactBranch()
		}
	}
	return deleted
}

func (n *node) flatten(items []item) []item {
	if !n.branch {
		items = append(items, n.items...)
	} else {
		for _, child := range n.nodes {
			if child.count > 0 {
				items = child.flatten(items)
			}
		}
	}
	return items
}

func (n *node) compactBranch() {
	n.items = n.flatten(nil)
	n.branch = false
	n.nodes = nil
}

// RemoveWhen removes an item from the tree based on it's cell and when the
// cond func returns true. It will delete at most a maximum of one item.
func (tr *Tree) RemoveWhen(cell uint64, cond func(data interface{}) bool) {
	if tr.root == nil {
		return
	}
	if tr.root.remove(cell, nil, 64-nBits, cond) {
		tr.count--
	}
}

// Scan iterates over the entire tree. Return false from iter function to stop.
func (tr *Tree) Scan(iter func(cell uint64, data interface{}) bool) {
	if tr.root == nil {
		return
	}
	tr.root.scan(iter)
}

func (n *node) scan(iter func(cell uint64, data interface{}) bool) bool {
	if !n.branch {
		for i := 0; i < len(n.items); i++ {
			if !iter(n.items[i].cell, n.items[i].data) {
				return false
			}
		}
	} else {
		for i := 0; i < len(n.nodes); i++ {
			if n.nodes[i].count > 0 {
				if !n.nodes[i].scan(iter) {
					return false
				}
			}
		}
	}
	return true
}

// Range iterates over the three start with the cell param.
func (tr *Tree) Range(
	pivot uint64,
	iter func(cell uint64, data interface{}) bool,
) {
	if tr.root != nil {
		tr.root.nodeRange(pivot, 64-nBits, false, iter)
	}
}

func (n *node) nodeRange(
	pivot uint64, bits uint, hit bool,
	iter func(cell uint64, data interface{}) bool,
) (hitout bool, ok bool) {
	if !n.branch {
		for _, item := range n.items {
			if hit || item.cell >= pivot {
				if !iter(item.cell, item.data) {
					return false, false
				}
			}
		}
		return true, true
	}
	index := 0
	if !hit {
		index = cellIndex(pivot, bits)
	}
	for ; index < len(n.nodes); index++ {
		hit, ok = n.nodes[index].nodeRange(pivot, bits-nBits, hit, iter)
		if !ok {
			return false, false
		}
	}
	return hit, true
}
