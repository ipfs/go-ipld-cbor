package cbornode

import (
	"testing"

	mh "github.com/multiformats/go-multihash"
)

type myStruct struct {
	items map[string]myStruct
	foo   string
	bar   []byte
	baz   []int
}

func init() {
	RegisterCborType(myStruct{})
}

func testStruct() myStruct {
	return myStruct{
		items: map[string]myStruct{
			"foo": {
				foo: "foo",
				bar: []byte("bar"),
				baz: []int{1, 2, 3, 4},
			},
			"bar": {
				bar: []byte("bar"),
				baz: []int{1, 2, 3, 4},
			},
		},
		baz: []int{5, 1, 2},
	}
}

func BenchmarkWrapObject(b *testing.B) {
	obj := testStruct()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nd, err := WrapObject(obj, mh.SHA2_256, -1)
		if err != nil {
			b.Fatal(err, nd)
		}
	}
}
func BenchmarkDecodeBlock(b *testing.B) {
	obj := testStruct()
	nd, err := WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		b.Fatal(err, nd)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nd2, err := DecodeBlock(nd)
		if err != nil {
			b.Fatal(err, nd2)
		}
	}
}
