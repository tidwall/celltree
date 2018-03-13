# `celltree`

[![GoDoc](https://img.shields.io/badge/api-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/tidwall/celltree)

A fast in-memory prefix tree that uses uint64 for keys, unsafe.Pointer for values, and allows for duplicate entries.

# Getting Started

### Installing

To start using celltree, install Go and run `go get`:

```sh
$ go get -u github.com/tidwall/celltree
```

## Example 

```go
var tr celltree.Tree

tr.Insert(10, nil, 0)
tr.Insert(5, nil, 0)
tr.Insert(31, nil, 0)
tr.Insert(16, nil, 0)
tr.Insert(9, nil, 0)

tr.Scan(func(cell uint64, value unsafe.Pointer, extra uint64) bool {
    println(cell)
    return true
})
```

Outputs:

```
5
9
10
16
31
```

## Performance

Single threaded performance comparing this package to
[google/btree](https://github.com/google/btree).

```
$ go test

-- celltree --
insert    1,048,576 ops in  318ms  3,296,579/sec
scan            100 ops in  824ms        121/sec
range     1,048,576 ops in  144ms  7,245,252/sec
remove    1,048,576 ops in  244ms  4,281,322/sec
memory    40,567,280 bytes 38/entry

-- btree --
insert    1,048,576 ops in 1003ms  1,044,876/sec
scan            100 ops in 1195ms         83/sec
range     1,048,576 ops in  443ms  2,364,467/sec
remove    1,048,576 ops in 1198ms    874,723/sec
memory    49,034,992 bytes 46/entry
```

*These benchmarks were run on a MacBook Pro 15" 2.8 GHz Intel Core i7 using Go 1.10*

## Contact

Josh Baker [@tidwall](http://twitter.com/tidwall)

## License

`celltree` source code is available under the MIT [License](/LICENSE).
