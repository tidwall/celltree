// Copyright 2018 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package celltree

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/google/btree"
	"github.com/tidwall/lotsa"
)

func init() {
	seed := (time.Now().UnixNano())
	//seed = 1544374208106411000
	println("seed:", seed)
	rand.Seed(seed)
}

func random(N int, perm bool) []uint64 {
	ints := make([]uint64, N)
	if perm {
		for i, x := range rand.Perm(N) {
			ints[i] = uint64(x)
		}
	} else {
		m := make(map[uint64]bool)
		for len(m) < N {
			m[rand.Uint64()] = true
		}
		var i int
		for k := range m {
			ints[i] = k
			i++
		}
	}
	return ints
}

func shuffle(ints []uint64) {
	for i := range ints {
		j := rand.Intn(i + 1)
		ints[i], ints[j] = ints[j], ints[i]
	}
}

func sortInts(ints []uint64) {
	sort.Slice(ints, func(i, j int) bool {
		return ints[i] < ints[j]
	})
}

func testEquals(t *testing.T, random, sorted []uint64) {
	t.Helper()
	sortInts(random)
	if len(sorted) != len(random) {
		t.Fatal("not equal")
	}
	for i := 0; i < len(sorted); i++ {
		if sorted[i] != random[i] {
			t.Fatal("not equal")
		}
	}
}

// sane tests the sanity of the tree. Any problems will panic.
func (tr *Tree) sane() {
	if tr.root == nil {
		if tr.count != 0 {
			panic(fmt.Sprintf("sane: expected %d, got %d", 0, tr.count))
		}
		return
	}
	count, _ := tr.root.saneCount(0, 64-numBits)
	if tr.count != count {
		panic(fmt.Sprintf("sane: expected %d, got %d", count, tr.count))
	}
}

func (n *node) saneCount(cell uint64, bits uint) (count int, cellout uint64) {
	if !n.branch {
		// all leaves count should match the number of items.
		if n.count != len(n.items) {
			panic(fmt.Sprintf("leaf has a count of %d, but %d items in array",
				n.count, len(n.items)))
		}
		// leaves should never go above max items unless they are at max depth.
		if n.count > maxItems && !maxDepth(bits) {
			panic(fmt.Sprintf("leaf has a count of %d, but maxItems is %d",
				n.count, maxItems))
		}
		// all leaves should not have non-nil leaves
		if len(n.items) == 0 && n.items != nil {
			panic(fmt.Sprintf("leaf has zero items, but a non-nil items array"))
		}
		// test if the items cells are in order
		for i := 0; i < len(n.items); i++ {
			if n.items[i].cell < cell {
				panic(fmt.Sprintf("leaf out of order at index: %d", i))
			}
			cell = n.items[i].cell
		}
		// all leaves should *not* fall below 40% capacity
		min := cap(n.items) * 40 / 100
		if len(n.items) <= min && len(n.items) > 0 {
			println(len(n.items), cap(n.items), min)
			panic(fmt.Sprintf("leaf is underfilled"))
		}
		return len(n.items), cell
	}
	// all branches should have a count of at least 1
	if n.count <= 0 {
		panic(fmt.Sprintf("branch has a count of %d", n.count))
	}
	// all branches should have a nil items array
	if n.items != nil {
		panic(fmt.Sprintf("branch has non-nil items"))
	}
	// check each node
	for i := 0; i < len(n.nodes); i++ {
		ncount, ncell := n.nodes[i].saneCount(cell, bits-numBits)
		count += ncount
		if ncell < cell {
			panic(fmt.Sprintf("branch out of order at index: %d", i))
		}
		cell = ncell
	}
	if n.count != count {
		panic(fmt.Sprintf("branch has wrong count, expected %d, got %d",
			count, n.count))
	}
	return count, cell
}

func TestRandomSingleStep(t *testing.T) {
	testRandomStep(t)
}

func testRandomStep(t *testing.T) {
	N := (rand.Int() % 10000)
	if N%2 == 1 {
		N++
	}
	ints := random(N, rand.Int()%2 == 0)
	var tr Tree
	for i := 0; i < N; i++ {
		tr.Insert(ints[i], nil)
		tr.sane()
	}
	if tr.Count() != N {
		t.Fatalf("expected %v, got %v", N, tr.Count())
	}
	var all []uint64
	tr.Scan(func(cell uint64, data interface{}) bool {
		all = append(all, cell)
		return true
	})
	testEquals(t, ints, all)
	if N > 0 {
		pivot := ints[len(ints)/2]
		var rangeCells []uint64
		tr.Range(pivot, func(cell uint64, data interface{}) bool {
			rangeCells = append(rangeCells, cell)
			return true
		})

		var scanCells []uint64
		tr.Scan(func(cell uint64, data interface{}) bool {
			if cell >= pivot {
				scanCells = append(scanCells, cell)
			}
			return true
		})
		testEquals(t, scanCells, rangeCells)
	}
	shuffle(ints)
	for i := 0; i < len(ints)/2; i++ {
		tr.Delete(ints[i], nil)
		tr.sane()
	}
	if tr.Count() != N/2 {
		t.Fatalf("expected %v, got %v", N/2, tr.Count())
	}
	for i := len(ints) / 2; i < len(ints); i++ {
		tr.Delete(ints[i], nil)
		tr.sane()
	}
	if tr.Count() != 0 {
		t.Fatalf("expected %v, got %v", 0, tr.Count())
	}
}
func TestRandom(t *testing.T) {
	start := time.Now()
	for time.Since(start) < time.Second {
		testRandomStep(t)
	}
}

func TestVarious(t *testing.T) {
	var tr Tree
	tr.Delete(0, nil)
	tr.DeleteWhen(0, nil)
	tr.Scan(nil)
	tr.Range(0, nil)
	tr.RangeDelete(0, math.MaxUint64, nil)

	N := 2000
	for i := 0; i < N; i++ {
		tr.Insert(uint64(i), i)
		tr.sane()
	}

	for i := 0; i < N; i++ {
		var j int
		tr.Scan(func(cell uint64, data interface{}) bool {
			if j == i {
				return false
			}
			j++
			return true
		})
	}

	for i := 0; i < N; i++ {
		var j int
		tr.Range(0, func(cell uint64, data interface{}) bool {
			if j == i {
				return false
			}
			j++
			return true
		})
	}

}

func TestDeleteWhen(t *testing.T) {
	var tr Tree
	tr.Insert(10, 0)
	tr.sane()
	tr.Insert(5, 1)
	tr.sane()
	tr.Insert(31, 2)
	tr.sane()
	tr.Insert(16, 3)
	tr.sane()
	tr.Insert(9, 4)
	tr.sane()
	tr.Insert(5, 5)
	tr.sane()
	tr.Insert(16, 6)
	tr.sane()
	var count int
	tr.DeleteWhen(16, func(data interface{}) bool {
		count++
		return false
	})
	if count != 2 {
		t.Fatalf("expected %v, got %v", 2, count)
	}
	if tr.Count() != 7 {
		t.Fatalf("expected %v, got %v", 7, tr.Count())
	}
	tr.DeleteWhen(16, func(data interface{}) bool {
		if data.(int) == 3 {
			return true
		}
		return false
	})
	if tr.Count() != 6 {
		t.Fatalf("expected %v, got %v", 6, tr.Count())
	}
}

type perfCtx struct {
	_insert func(cell uint64)
	_count  func() int
	_scan   func()
	_range  func(cell uint64, iter func(cell uint64) bool)
	_remove func(cell uint64)
}

type btreeItem uint64

func (v btreeItem) Less(v2 btree.Item) bool {
	return v < v2.(btreeItem)
}

func printPerfLabel(label string, randomized, shuffled bool) {
	print("-- " + label + " (")
	if randomized {
		print("randomized")
	} else {
		print("sequential")
	}
	if shuffled {
		print(",shuffled")
	} else {
		print(",ordered")
	}
	println(") --")
}
func TestPerf(t *testing.T) {
	if os.Getenv("BASICPERF") != "1" {
		fmt.Printf("TestPerf disabled (BASICPERF=1)\n")
		return
	}

	// CellTree
	for i := 0; i < 4; i++ {
		randomized := i/2 == 0
		shuffled := i%2 == 0
		t.Run("CellTree", func(t *testing.T) {
			printPerfLabel("celltree", randomized, shuffled)
			var tr Tree
			ctx := perfCtx{
				_insert: func(cell uint64) { tr.Insert(cell, nil) },
				_count:  func() int { return tr.Count() },
				_scan: func() {
					tr.Scan(func(cell uint64, data interface{}) bool {
						return true
					})
				},
				_range: func(cell uint64, iter func(cell uint64) bool) {
					tr.Range(cell, func(cell uint64, data interface{}) bool {
						return iter(cell)
					})
				},
				_remove: func(cell uint64) { tr.Delete(cell, nil) },
			}
			testPerf(t, ctx, randomized, shuffled)
		})
	}

	// BTree
	for i := 0; i < 4; i++ {
		randomized := i/2 == 0
		shuffled := i%2 == 0
		t.Run("BTree", func(t *testing.T) {
			printPerfLabel("btree", randomized, shuffled)
			tr := btree.New(16)
			ctx := perfCtx{
				_insert: func(cell uint64) { tr.ReplaceOrInsert(btreeItem(cell)) },
				_count:  func() int { return tr.Len() },
				_scan: func() {
					tr.Ascend(func(item btree.Item) bool {
						return true
					})
				},
				_range: func(cell uint64, iter func(cell uint64) bool) {
					tr.AscendGreaterOrEqual(btreeItem(cell), func(item btree.Item) bool {
						return iter(uint64(item.(btreeItem)))
					})
				},
				_remove: func(cell uint64) { tr.Delete(btreeItem(cell)) },
			}
			testPerf(t, ctx, randomized, shuffled)
		})
	}
}

func testPerf(t *testing.T, ctx perfCtx, randomozed, shuffled bool) {
	N := 1024 * 1024
	var ints []uint64
	if randomozed {
		ints = random(N, false)
	} else {
		start := rand.Uint64()
		for i := 0; i < N; i++ {
			ints = append(ints, start+uint64(i))
		}
	}
	if shuffled {
		shuffle(ints)
	} else {
		sort.Slice(ints, func(i, j int) bool {
			return ints[i] < ints[j]
		})
	}

	var ms1, ms2 runtime.MemStats
	defer func() {
		heapBytes := int(ms2.HeapAlloc - ms1.HeapAlloc)
		fmt.Printf("memory %13s bytes %s/entry \n",
			commaize(heapBytes), commaize(heapBytes/len(ints)))
		fmt.Printf("\n")
	}()
	runtime.GC()
	time.Sleep(time.Millisecond * 100)
	runtime.ReadMemStats(&ms1)

	var start time.Time
	var dur time.Duration
	output := func(tag string, N int) {
		dur = time.Since(start)
		fmt.Printf("%-8s %10s ops in %4dms %10s/sec\n",
			tag, commaize(N), int(dur.Seconds()*1000),
			commaize(int(float64(N)/dur.Seconds())))
	}

	/////////////////////////////////////////////
	start = time.Now()
	lotsa.Ops(N, 1, func(i, _ int) {
		ctx._insert(ints[i])
	})
	output("insert", N)
	runtime.GC()
	time.Sleep(time.Millisecond * 100)
	runtime.ReadMemStats(&ms2)

	if ctx._count() != N {
		t.Fatalf("expected %v, got %v", N, ctx._count())
	}
	/////////////////////////////////////////////
	shuffle(ints)
	start = time.Now()
	lotsa.Ops(100, 1, func(i, _ int) { ctx._scan() })
	output("scan", 100)
	/////////////////////////////////////////////
	sortInts(ints)
	start = time.Now()
	lotsa.Ops(N, 1, func(i, _ int) {
		var found bool
		ctx._range(ints[i], func(cell uint64) bool {
			if cell != ints[i] {
				t.Fatal("invalid")
			}
			found = true
			return false
		})
		if !found {
			t.Fatal("not found")
		}
	})
	output("range", N)
	/////////////////////////////////////////////
	shuffle(ints)
	start = time.Now()
	lotsa.Ops(N, 1, func(i, _ int) {
		ctx._remove(ints[i])
	})
	output("remove", N)
	if ctx._count() != 0 {
		t.Fatalf("expected %v, got %v", 0, ctx._count())
	}
}

func commaize(n int) string {
	s1, s2 := fmt.Sprintf("%d", n), ""
	for i, j := len(s1)-1, 0; i >= 0; i, j = i-1, j+1 {
		if j%3 == 0 && j != 0 {
			s2 = "," + s2
		}
		s2 = string(s1[i]) + s2
	}
	return s2
}

func TestPerfLongTime(t *testing.T) {
	if os.Getenv("PERFLONGTIME") != "1" {
		fmt.Printf("TestPerfLongTime disabled (PERFLONGTIME=1)\n")
		return
	}
	x := 0
	N := 1024 * 1024
	ints := random(N, true)
	var tr Tree
	var insops, remops int
	var ms1, ms2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms1)
	start := time.Now()
	var insdur, remdur time.Duration
	var xstart time.Time

	// insert all items

	for i := 0; i < len(ints); i++ {
		tr.Insert(ints[i], nil)
		insops++
	}
	insdur += time.Since(start)
	// now delete every 4th item and rerandomize
	for {
		opp := rand.Uint64()
		xstart = time.Now()
		for i := x; i < len(ints); i += 4 {
			tr.Delete(ints[i], nil)
			ints[i] ^= opp
			remops++
		}
		remdur += time.Since(xstart)
		xstart = time.Now()
		for i := x; i < len(ints); i += 4 {
			tr.Insert(ints[i], nil)
			insops++
		}
		insdur += time.Since(xstart)
		if tr.Count() != N {
			t.Fatal("shit")
		}
		runtime.GC()
		runtime.ReadMemStats(&ms2)
		heapBytes := int(ms2.HeapAlloc - ms1.HeapAlloc)
		x = (x + 1) % 4
		dur := time.Since(start)

		fmt.Printf("\r  %10s ops %10s ins/sec %10s rem/sec (%s bytes/cell)\r",
			commaize(insops+remops),
			commaize(int(float64(insops)/insdur.Seconds())),
			commaize(int(float64(remops)/remdur.Seconds())),
			commaize(heapBytes/N),
		)
		if dur > time.Minute {
			break
		}
	}
	fmt.Printf("\n")
}

func TestDupCells(t *testing.T) {
	N := 1000000
	cell := uint64(388098102398102938)
	var tr Tree
	for i := 0; i < N; i++ {
		tr.Insert(cell, i)
	}
}

func cellsEqual(cells1, cells2 []uint64) bool {
	if len(cells1) != len(cells2) {
		return false
	}
	for i := 0; i < len(cells1); i++ {
		if cells1[i] != cells2[i] {
			return false
		}
	}
	return true
}

func TestRange(t *testing.T) {
	N := 10000
	start := uint64(10767499590539539808)

	var tr Tree
	for i := 0; i < N; i++ {
		cell := start + uint64(i)
		tr.Insert(cell, cell)
		tr.sane()
	}
	var count int
	var cells1 []uint64
	tr.Scan(func(cell uint64, value interface{}) bool {
		cells1 = append(cells1, cell)
		count++
		return true
	})
	if count != N {
		t.Fatalf("expected %v, got %v", N, count)
	}
	count = 0
	var cells2 []uint64
	tr.Range(0, func(cell uint64, value interface{}) bool {
		cells2 = append(cells2, cell)
		count++
		return true
	})
	if count != N {
		t.Fatalf("expected %v, got %v", N, count)
	}
	if !cellsEqual(cells1, cells2) {
		t.Fatal("not equal")
	}

	// random ranges over some random data

	var all []uint64
	tr = Tree{}
	for i := 0; i < N; i++ {
		cell := rand.Uint64()
		all = append(all, cell)
		tr.Insert(cell, cell)
		tr.sane()
	}
	sortInts(all)

	for i := 0; i < 100; i++ {
		min := rand.Uint64()
		max := rand.Uint64()
		if min > max {
			min, max = max, min
		}
		var hits1 []uint64
		tr.Range(min, func(cell uint64, value interface{}) bool {
			if cell < min {
				t.Fatalf("cell %v is less than %v", cell, min)
			}
			if cell > max {
				return false
			}
			hits1 = append(hits1, cell)
			return true
		})
		var hits2 []uint64
		for _, cell := range all {
			if cell >= min && cell <= max {
				hits2 = append(hits2, cell)
			}
		}
		if !cellsEqual(hits1, hits2) {
			t.Fatal("cells not equal")
		}
	}

}

func TestDuplicates(t *testing.T) {
	N := 10000
	var tr Tree
	for i := 0; i < N; i++ {
		tr.Insert(uint64(i), i)
		tr.sane()
	}
	tr.InsertOrReplace(5000, 5000,
		func(data interface{}) (newData interface{}, replace bool) {
			return nil, false
		},
	)
	tr.sane()
	if tr.Count() != N+1 {
		t.Fatalf("expected %v, got %v", N+1, tr.Count())
	}
	tr.InsertOrReplace(2500, 2500,
		func(data interface{}) (newData interface{}, replace bool) {
			return "hello", true
		},
	)
	tr.sane()
	if tr.Count() != N+1 {
		t.Fatalf("expected %v, got %v", N+1, tr.Count())
	}

	for i := 0; i < 500; i++ {
		tr.InsertOrReplace(uint64(i)+10000000, i,
			func(data interface{}) (newData interface{}, replace bool) {
				if i%2 == 0 {
					return nil, false
				}
				return nil, false
			},
		)
		tr.sane()
	}
}

func TestRangeDelete(t *testing.T) {
	N := 1000
	start := 5000
	var tr Tree
	var cells1 []uint64
	for i := 0; i < N; i++ {
		cell := uint64(start + i)
		tr.Insert(cell, i)
		tr.sane()
		cells1 = append(cells1, cell)
	}
	if tr.Count() != N {
		t.Fatalf("expected %v, got %v", N, tr.Count())
	}

	// starting from the second half, do not delete any
	var count int
	tr.RangeDelete(uint64(start+N/2), math.MaxUint64,
		func(cell uint64, value interface{}) (shouldDelete, ok bool) {
			count++
			return false, true
		},
	)
	if count != N/2 {
		t.Fatalf("expected %v, got %v", N/2, count)
	}
	if tr.Count() != N {
		t.Fatalf("expected %v, got %v", N, tr.Count())
	}

	// delete the last half of the items
	count = 0
	tr.RangeDelete(uint64(start+N/2), math.MaxUint64,
		func(cell uint64, value interface{}) (shouldDelete, ok bool) {
			count++
			return true, true
		},
	)
	if count != N/2 {
		t.Fatalf("expected %v, got %v", N/2, count)
	}
	if tr.Count() != N/2 {
		t.Fatalf("expected %v, got %v", N/2, tr.Count())
	}

	var cells2 []uint64
	tr.Scan(func(cell uint64, value interface{}) bool {
		cells2 = append(cells2, cell)
		return true
	})
	if !cellsEqual(cells1[:N/2], cells2) {
		t.Fatal("not equal")
	}
}

func TestRandomRangeDelete(t *testing.T) {
	var count int
	start := time.Now()
	for time.Since(start) < time.Second {
		TestSingleRandomRangeDelete(t)
		count++
	}
}
func TestSingleRandomRangeDelete(t *testing.T) {
	// random ranges over some random data and randomly cells
	N := rand.Int() % 50000
	if N%2 == 1 {
		N++
	}
	if N < 2 {
		N = 2
	}
	var all []uint64
	var tr Tree
	switch rand.Int() % 5 {
	case 0:
		// random and spread out
		for i := 0; i < N; i++ {
			cell := rand.Uint64()
			all = append(all, cell)
			tr.Insert(cell, cell)
		}
	case 1:
		// random and centered
		for i := 0; i < N; i++ {
			cell := rand.Uint64()/2 + math.MaxUint64/4
			all = append(all, cell)
			tr.Insert(cell, cell)
		}
	case 2:
		// random and left-aligned
		for i := 0; i < N; i++ {
			cell := rand.Uint64() / 2
			all = append(all, cell)
			tr.Insert(cell, cell)
		}
	case 3:
		// random and right-aligned
		for i := 0; i < N; i++ {
			cell := rand.Uint64()/2 + math.MaxInt64/2
			all = append(all, cell)
			tr.Insert(cell, cell)
		}
	case 4:
		// sequential and centered
		for i := 0; i < N; i++ {
			cell := math.MaxUint64/2 - uint64(N/2+i)
			all = append(all, cell)
			tr.Insert(cell, cell)
		}
	}
	sortInts(all)
	deletes := make(map[uint64]bool)
	for _, cell := range all {
		deletes[cell] = rand.Int()%2 == 0
	}
	var min, max uint64
	switch rand.Uint64() % 4 {
	case 0:
		// start at zero
		min = 0
	case 1:
		// start before first
		min = all[0] / 2
	case 2:
		// start on the first
		min = all[0]
	case 3:
		// start in random position
		min = rand.Uint64()
	}
	switch rand.Uint64() % 4 {
	case 0:
		// end at max uint64
		min = math.MaxUint64
	case 1:
		// end after last
		min = (math.MaxUint64-all[len(all)-1])/2 + all[len(all)-1]
	case 2:
		// end on the last
		min = all[len(all)-1]
	case 3:
		// end in random position
		min = rand.Uint64()
	}
	if min > max {
		min, max = max, min
	}

	// if min == 0 {
	// 	println(min == 0, tr.root.nodes[0].count)
	// }

	var hits1 []uint64
	tr.RangeDelete(min, max,
		func(cell uint64, value interface{}) (shouldDelete bool, ok bool) {
			if cell < min {
				t.Fatalf("cell %v is less than %v", cell, min)
			}
			var ok2 bool
			shouldDelete, ok2 = deletes[cell]
			if !ok2 {
				t.Fatal("missing cell in deletes map")
			}
			if shouldDelete {
				// delete this item
				hits1 = append(hits1, cell)
			}
			return shouldDelete, true
		},
	)
	tr.sane()
	var hits2 []uint64
	for _, cell := range all {
		if cell >= min && cell <= max && deletes[cell] {
			hits2 = append(hits2, cell)
		}
	}
	if !cellsEqual(hits1, hits2) {
		t.Fatal("cells not equal")
	}
}

func testRangeDeleteNoIterator(t *testing.T, N int) {
	var tr Tree
	var all []uint64
	for i := 0; i < N; i++ {
		cell := rand.Uint64()
		all = append(all, cell)
		tr.Insert(cell, nil)
	}
	sortInts(all)
	start := uint64(math.MaxUint64 / 4)
	end := start + math.MaxUint64/2
	tr.RangeDelete(start, end, nil)
	tr.sane()
	var cells1 []uint64
	tr.Scan(func(cell uint64, _ interface{}) bool {
		cells1 = append(cells1, cell)
		return true
	})
	var cells2 []uint64
	for _, cell := range all {
		if cell < start || cell > end {
			cells2 = append(cells2, cell)
		}
	}
	if !cellsEqual(cells1, cells2) {
		t.Fatal("not equal")
	}
}

func TestRangeDeleteNoIterator(t *testing.T) {
	testRangeDeleteNoIterator(t, 0)
	testRangeDeleteNoIterator(t, maxItems/2)
	testRangeDeleteNoIterator(t, maxItems-1)
	testRangeDeleteNoIterator(t, maxItems)
	testRangeDeleteNoIterator(t, maxItems+1)
	testRangeDeleteNoIterator(t, 100000)
}
