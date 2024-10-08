package cbornode

import (
	"sync"
	"testing"

	mh "github.com/multiformats/go-multihash"
)

type MyStruct struct {
	Items map[string]MyStruct
	Foo   string
	Bar   []byte
	Baz   []int
}

func init() {
	RegisterCborType(MyStruct{})
}

func testStruct() MyStruct {
	return MyStruct{
		Items: map[string]MyStruct{
			"Foo": {
				Foo: "Foo",
				Bar: []byte("Bar"),
				Baz: []int{1, 2, 3, 4},
			},
			"Bar": {
				Bar: []byte("Bar"),
				Baz: []int{1, 2, 3, 4},
			},
		},
		Baz: []int{5, 1, 2},
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

func BenchmarkWrapObjectParallel(b *testing.B) {
	obj := testStruct()
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(100)
	errors := make(chan error, 100)
	for j := 0; j < 100; j++ {
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				if _, err := WrapObject(obj, mh.SHA2_256, -1); err != nil {
					errors <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errors)
	for e := range errors {
		b.Fatal(e)
	}
}

func BenchmarkDecodeBlockParallel(b *testing.B) {
	obj := testStruct()
	nd, err := WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		b.Fatal(err, nd)
	}
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(100)
	errs := make(chan error, 100)
	for j := 0; j < 100; j++ {
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				if _, err := DecodeBlock(nd); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		b.Fatal(e)
	}
}

func BenchmarkEncode(b *testing.B) {
	obj := testStruct()
	for i := 0; i < b.N; i++ {
		bytes, err := Encode(obj)
		if err != nil {
			b.Fatal(err, bytes)
		}
	}
}
