// Copyright 2018 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package celltree

import "unsafe"

const maxItems = 256 // max items per node
const nBits = 8      // 1, 2,  4,   8  - match nNodes with the correct nBits
const nNodes = 256   // 2, 4, 16, 256  - match nNodes with the correct nBits

type cellT struct {
	id    uint64
	extra uint64
	data  unsafe.Pointer
}

type nodeT struct {
	branch bool
	items  []cellT
	nodes  []*nodeT
}

// Tree is a uint64 prefix tree
type Tree struct {
	len  int
	root *nodeT
}

// Insert inserts an item into the tree. Items are ordered by it's cell.
// The extra param is a simple user context value.
func (tr *Tree) Insert(cell uint64, data unsafe.Pointer, extra uint64) {
	if tr.root == nil {
		tr.root = new(nodeT)
	}
	tr.insert(tr.root, cell, data, extra, 64-nBits)
	tr.len++
}

func (tr *Tree) insert(n *nodeT, cell uint64, data unsafe.Pointer, extra uint64, bits uint) {
	if !n.branch {
		if bits == 0 || len(n.items) < maxItems {
			i := tr.find(n, cell)
			n.items = append(n.items, cellT{})
			copy(n.items[i+1:], n.items[i:len(n.items)-1])
			n.items[i] = cellT{id: cell, extra: extra, data: data}
			return
		}
		tr.split(n, bits)
		tr.insert(n, cell, data, extra, bits)
		return
	}
	i := int(cell >> bits & (nNodes - 1))
	for i >= len(n.nodes) {
		n.nodes = append(n.nodes, nil)
	}
	if n.nodes[i] == nil {
		n.nodes[i] = new(nodeT)
	}
	tr.insert(n.nodes[i], cell, data, extra, bits-nBits)
}

// Len returns the number of items in the tree.
func (tr *Tree) Len() int {
	return tr.len
}

func (tr *Tree) split(n *nodeT, bits uint) {
	n.branch = true
	for i := 0; i < len(n.items); i++ {
		tr.insert(n, n.items[i].id, n.items[i].data, n.items[i].extra, bits)
	}
	n.items = nil
}

func (tr *Tree) find(n *nodeT, cell uint64) int {
	i, j := 0, len(n.items)
	for i < j {
		h := i + (j-i)/2
		if !(cell < n.items[h].id) {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}

// Remove removes an item from the tree based on it's cell and data values.
func (tr *Tree) Remove(cell uint64, data unsafe.Pointer) {
	if tr.root == nil {
		return
	}
	if tr.remove(tr.root, cell, data, 64-nBits) {
		tr.len--
	}
}

func (tr *Tree) remove(n *nodeT, cell uint64, data unsafe.Pointer, bits uint) bool {
	if !n.branch {
		i := tr.find(n, cell) - 1
		for ; i >= 0; i-- {
			if n.items[i].id != cell {
				break
			}
			if n.items[i].data == data {
				n.items[i] = cellT{}
				copy(n.items[i:len(n.items)-1], n.items[i+1:])
				n.items = n.items[:len(n.items)-1]
				return true
			}
		}
		return false
	}
	i := int(cell >> bits & (nNodes - 1))
	if i >= len(n.nodes) || n.nodes[i] == nil ||
		!tr.remove(n.nodes[i], cell, data, bits-nBits) {
		return false
	}
	if !n.nodes[i].branch && len(n.nodes[i].items) == 0 {
		n.nodes[i] = nil
		for i := 0; i < len(n.nodes); i++ {
			if n.nodes[i] != nil {
				return true
			}
		}
		n.branch = false
	}
	return true
}

// Scan iterates over the entire tree. Return false from the iter function to stop.
func (tr *Tree) Scan(iter func(cell uint64, data unsafe.Pointer, extra uint64) bool) {
	if tr.root == nil {
		return
	}
	tr.scan(tr.root, iter)
}

func (tr *Tree) scan(n *nodeT, iter func(cell uint64, data unsafe.Pointer, extra uint64) bool) bool {
	if !n.branch {
		for i := 0; i < len(n.items); i++ {
			if !iter(n.items[i].id, n.items[i].data, n.items[i].extra) {
				return false
			}
		}
	} else {
		for i := 0; i < len(n.nodes); i++ {
			if n.nodes[i] != nil {
				if !tr.scan(n.nodes[i], iter) {
					return false
				}
			}
		}
	}
	return true
}

// Range iterates over the three start with the cell param.
func (tr *Tree) Range(cell uint64, iter func(cell uint64, key unsafe.Pointer, extra uint64) bool) {
	if tr.root == nil {
		return
	}
	tr._range(tr.root, cell, 64-nBits, iter)
}

func (tr *Tree) _range(n *nodeT, cell uint64, bits uint, iter func(cell uint64, data unsafe.Pointer, extra uint64) bool) (hit, ok bool) {
	if !n.branch {
		hit = true
		i := tr.find(n, cell) - 1
		for ; i >= 0; i-- {
			if n.items[i].id < cell {
				break
			}
		}
		i++
		for ; i < len(n.items); i++ {
			if !iter(n.items[i].id, n.items[i].data, n.items[i].extra) {
				return hit, false
			}
		}
		return hit, true
	}
	i := int(cell >> bits & (nNodes - 1))
	if i >= len(n.nodes) || n.nodes[i] == nil {
		return hit, true
	}
	for ; i < len(n.nodes); i++ {
		if n.nodes[i] != nil {
			if hit {
				if !tr.scan(n.nodes[i], iter) {
					return hit, false
				}
			} else {
				hit, ok = tr._range(n.nodes[i], cell, bits-nBits, iter)
				if !ok {
					return hit, false
				}
			}
		}
	}
	return hit, true
}
