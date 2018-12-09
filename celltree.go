// Copyright 2018 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package celltree

const (
	numBits  = 7   // [1,2,3,4...8]    match numNodes with the correct numBits
	numNodes = 128 // [2,4,8,16...256] match numNodes with the correct numBits
)
const (
	maxItems = 256                 // max num of items in a leaf
	minItems = maxItems * 40 / 100 // min num of items in a branch
)

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

// Count returns the number of items in the tree.
func (tr *Tree) Count() int {
	return tr.count
}

func cellIndex(cell uint64, bits uint) int {
	return int(cell >> bits & uint64(numNodes-1))
}

// InsertOrReplace inserts an item into the tree. Items are ordered by it's
// cell. The extra param is a simple user context value. The cond function is
// used to allow for replacing an existing cell with a new cell. When the
// 'replace' return value is set to false, then the original data is inserted.
// When the 'replace' value is true the existing cell data is replace with
// newData.
func (tr *Tree) InsertOrReplace(
	cell uint64, data interface{},
	cond func(data interface{}) (newData interface{}, replace bool),
) {
	if tr.root == nil {
		tr.root = new(node)
	}
	if tr.root.insert(cell, data, 64-numBits, cond) {
		tr.count++
	}
}

// Insert inserts an item into the tree. Items are ordered by it's cell.
// The extra param is a simple user context value.
func (tr *Tree) Insert(cell uint64, data interface{}) {
	tr.InsertOrReplace(cell, data, nil)
}

func (n *node) splitLeaf(bits uint) {
	n.branch = true
	// reset the node count to zero
	n.count = 0
	// create space for all of the nodes
	n.nodes = make([]node, numNodes)
	// reinsert all of leaf items
	for i := 0; i < len(n.items); i++ {
		n.insert(n.items[i].cell, n.items[i].data, bits, nil)
	}
	// release the leaf items
	n.items = nil
}

func maxDepth(bits uint) bool {
	return bits < numBits
}

func (n *node) insert(
	cell uint64, data interface{}, bits uint,
	cond func(data interface{}) (newData interface{}, replace bool),
) (inserted bool) {
	if !n.branch {
		// leaf node
		atcap := !maxDepth(bits) && len(n.items) >= maxItems
	insertAgain:
		if atcap && cond == nil {
			// split leaf. it's at capacity
			n.splitLeaf(bits)
			// insert item again, but this time node is a branch
			n.insert(cell, data, bits, nil)
			// we need to deduct one item from the count, otherwise it'll be
			// the target cell will be counted twice
			n.count--
		} else {
			// find the target index for the new cell
			if len(n.items) == 0 || n.items[len(n.items)-1].cell < cell {
				// the new cell is greater than the last cell in leaf, so
				// we can just append it
				if atcap {
					cond = nil
					goto insertAgain
				}
				n.items = append(n.items, item{cell: cell, data: data})
			} else {
				// locate the index of the cell in the leaf
				index := n.findLeafItem(cell)
				if cond != nil {
					// find a duplicate cell
					for i := index - 1; i >= 0; i-- {
						if n.items[i].cell != cell {
							// did not find
							break
						}
						// found a duplicate
						newData, replace := cond(n.items[i].data)
						if replace {
							// must replace the cell data instead of inserting
							// a new one.
							n.items[i].data = newData
							return false
						}
					}
					// condition func was not safisfied. this means that the
					// new item will be inserted/
					if atcap {
						cond = nil
						goto insertAgain
					}
				}
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
		if !n.nodes[index].insert(cell, data, bits-numBits, cond) {
			return false
		}
	}
	// increment the node
	n.count++
	return true
}

// find an index of the cell using a binary search
func (n *node) findLeafItem(cell uint64) int {
	i, j := 0, len(n.items)
	for i < j {
		h := i + (j-i)/2
		if cell >= n.items[h].cell {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}

// Delete removes an item from the tree based on it's cell and data values.
func (tr *Tree) Delete(cell uint64, data interface{}) {
	if tr.root == nil {
		return
	}
	if tr.root.nodeDelete(cell, data, 64-numBits, nil) {
		tr.count--
	}
}

func (n *node) nodeDelete(
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
				if len(n.items) == 1 {
					// do not have non-nil leaves hanging around
					n.items = nil
				} else {
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
				}
				deleted = true
				break
			}
		}
	} else {
		// branch node
		index := cellIndex(cell, bits)
		deleted = n.nodes[index].nodeDelete(cell, data, bits-numBits, cond)
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

// DeleteWhen removes an item from the tree based on it's cell and when the
// cond func returns true. It will delete at most a maximum of one item.
func (tr *Tree) DeleteWhen(cell uint64, cond func(data interface{}) bool) {
	if tr.root == nil {
		return
	}
	if tr.root.nodeDelete(cell, nil, 64-numBits, cond) {
		tr.count--
	}
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
	n.count = len(n.items)
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

// Range iterates over the tree starting with the pivot param.
func (tr *Tree) Range(
	pivot uint64,
	iter func(cell uint64, data interface{}) bool,
) {
	if tr.root != nil {
		tr.root.nodeRange(pivot, 64-numBits, false, iter)
	}
}

func (n *node) nodeRange(
	pivot uint64, bits uint, hit bool,
	iter func(cell uint64, data interface{}) bool,
) (hitout bool, ok bool) {
	if !n.branch {
		for _, item := range n.items {
			if item.cell < pivot {
				continue
			}
			if !iter(item.cell, item.data) {
				return false, false
			}
		}
		return true, true
	}
	var index int
	if hit {
		index = 0
	} else {
		index = cellIndex(pivot, bits)
	}
	for ; index < len(n.nodes); index++ {
		if n.nodes[index].count == 0 {
			hit = true
		} else {
			hit, ok = n.nodes[index].nodeRange(pivot, bits-numBits, hit, iter)
			if !ok {
				return false, false
			}
		}
	}
	return hit, true
}

// RangeDelete iterates over the tree starting with the pivot param and "asks"
// the iterator if the item should be deleted.
func (tr *Tree) RangeDelete(
	start, end uint64,
	iter func(cell uint64, data interface{}) (shouldDelete bool, ok bool),
) {
	if tr.root == nil {
		return
	}
	_, deleted, _ := tr.root.nodeRangeDelete(
		start, end, 64-numBits, 0, false, iter)
	tr.count -= deleted
}

func (n *node) nodeRangeDelete(
	start, end uint64, bits uint, base uint64, hit bool,
	iter func(cell uint64, data interface{}) (shouldDelete bool, ok bool),
) (hitout bool, deleted int, ok bool) {
	if !n.branch {
		ok = true
		var skipIterator bool
		if iter == nil && len(n.items) > 0 {
			if n.items[0].cell >= start &&
				n.items[len(n.items)-1].cell <= end {
				// clear the entire leaf
				deleted = len(n.items)
				skipIterator = true
			}
		}
		for i := 0; !skipIterator && i < len(n.items); i++ {
			if n.items[i].cell < start {
				continue
			}
			var shouldDelete bool
			if ok {
				// ask if the current item should be deleted and/or if the
				// iterator should stop.
				if n.items[i].cell > end {
					// past the end, don't delete and don't continue
					ok = false
				} else {
					if iter == nil {
						shouldDelete = true
					} else {
						shouldDelete, ok = iter(
							n.items[i].cell, n.items[i].data)
					}
				}
			} else {
				// a previous iterator requested to stop, so do not delete
				// the current item.
				shouldDelete = false
			}
			if shouldDelete {
				// should delete item. increment the delete counter
				deleted++
			} else {
				// should keep item.
				if deleted > 0 {
					// there's room in a previously deleted slot, move the
					// current item there.
					n.items[i-deleted] = n.items[i]
					n.items[i].data = nil
				} else if !ok {
					// the iterate requested a stop and since there's no
					// deleted items, we can immediately stop here.
					break
				}
			}
		}
		if deleted > 0 {
			// there was some deleted items so we need to adjust the length
			// of the items array to reflect the change
			n.items = n.items[:len(n.items)-deleted]
			if len(n.items) == 0 {
				n.items = nil
			} else {
				// check if the base array needs to be shrunk/reallocated.
				ncap := cap(n.items)
				min := ncap * 40 / 100
				if len(n.items) <= min {
					for len(n.items) <= min {
						ncap /= 2
						min = ncap * 40 / 100
					}
					// shrink and realloc the array
					items := make([]item, len(n.items), ncap)
					copy(items, n.items)
					n.items = items
				}
			}
		}
		// set the hit flag once a leaf is reached
		hit = true
	} else {
		var index int
		if hit {
			// target leaf node has been reached. this means we can just start at
			// index zero and expect that all of the following noes are candidates.
			index = 0
		} else {
			// target leaf node has not been reached yet so we need to determine
			// the best path to get to it.
			index = cellIndex(start, bits)
		}
		for ; index < len(n.nodes); index++ {
			if n.nodes[index].count == 0 {
				hit = true
			} else {
				var dropped bool
				if hit && iter == nil {
					cellStart := (base + uint64(index)) << bits
					cellEnd := ((base + uint64(index+1)) << bits) - 1
					// we've already hit a leaf and the iter is nil. It's
					// possible that this entire node can be deleted if it's
					// cell range fits within start/end.
					if cellStart >= start && cellEnd <= end {
						// drop the node altogether
						deleted += n.nodes[index].count
						n.nodes[index] = node{}
						dropped = true
					}
				}
				if !dropped {
					var ndeleted int
					hit, ndeleted, ok = n.nodes[index].nodeRangeDelete(
						start, end, bits-numBits,
						(base<<numBits)+uint64(index),
						hit, iter)
					deleted += ndeleted
					if !ok {
						break
					}
				}
			}
		}
	}
	if deleted > 0 {
		// an item was deleted from this node or a child node
		// decrement the counter
		n.count -= deleted
		if n.branch && n.count <= minItems {
			// compact the branch into a leaf
			n.compactBranch()
		}
	}
	return hit, deleted, ok
}
