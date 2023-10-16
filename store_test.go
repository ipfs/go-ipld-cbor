package cbornode_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ipfs/boxo/blockstore"
	u "github.com/ipfs/boxo/util"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	dstest "github.com/ipfs/go-datastore/test"
	cbornode "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
)

var bg = context.Background()

func TestGetWhenKeyNotPresent(t *testing.T) {
	bs := blockstore.NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	cborStore := cbornode.NewCborStore(bs)
	c := cid.NewCidV1(uint64(cid.DagCBOR), u.Hash([]byte("stuff")))
	out := new(cbornode.CborByteArray)
	err := cborStore.Get(bg, c, out)

	if !bytes.Equal([]byte{}, []byte(*out)) {
		t.Error("nil block expected")
	}
	if err == nil {
		t.Error("error expected, got nil")
	}
}

func TestGetWhenKeyIsNil(t *testing.T) {
	bs := blockstore.NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	cborStore := cbornode.NewCborStore(bs)
	out := new(cbornode.CborByteArray)
	err := cborStore.Get(bg, cid.Cid{}, out)

	if !bytes.Equal([]byte{}, []byte(*out)) {
		t.Error("nil block expected")
	}
	if err == nil {
		t.Error("error expected, got nil")
	}
}

func TestPutThenGetBlock(t *testing.T) {
	bs := blockstore.NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	cborStore := cbornode.NewCborStore(bs)
	cborObj := cbornode.Cborstr("some data")

	c, err := cborStore.Put(bg, cborObj)
	if err != nil {
		t.Fatal(err)
	}
	if !cborObj.Cid().Equals(c) {
		t.Fatalf("expected cid %s does not match actual cid %s", cborObj.Cid().String(), c.String())
	}

	out := new(cbornode.CborByteArray)
	err = cborStore.Get(bg, cborObj.Cid(), out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cborObj.RawData(), out.RawData()) {
		t.Fatalf("expected result %x does not match actual result %x", cborObj.RawData(), out.RawData())
	}
}

func TestGetManyWhenKeyNotPresent(t *testing.T) {
	bs := blockstore.NewGetManyBlockstore(dstest.NewTestTxnDatastore(ds.NewMapDatastore(), false))
	cborStore := cbornode.NewGetManyCborStore(bs)
	c1 := cid.NewCidV1(uint64(cid.DagCBOR), u.Hash([]byte("stuff")))
	c2 := cid.NewCidV1(uint64(cid.DagCBOR), u.Hash([]byte("stuff2")))
	outs := []interface{}{new(cbornode.CborByteArray), new(cbornode.CborByteArray)}
	cursors, missingCIDs, err := cborStore.GetMany(bg, []cid.Cid{c1, c2}, outs)
	if len(cursors) != 0 {
		t.Error("no cursors expected")
	}
	if len(missingCIDs) != 2 {
		t.Error("2 missing cids expected")
	}
	if err != nil {
		t.Error("no error expected")
	}
}

func TestGetManyWhenKeyIsNil(t *testing.T) {
	bs := blockstore.NewGetManyBlockstore(dstest.NewTestTxnDatastore(ds.NewMapDatastore(), false))
	cborStore := cbornode.NewGetManyCborStore(bs)
	outs := []interface{}{new(cbornode.CborByteArray), new(cbornode.CborByteArray)}
	_, _, err := cborStore.GetMany(bg, []cid.Cid{{}, {}}, outs)
	if !ipld.IsNotFound(err) {
		t.Fail()
	}
}

func TestPutManyThenGetManyBlock(t *testing.T) {
	bs := blockstore.NewGetManyBlockstore(dstest.NewTestTxnDatastore(ds.NewMapDatastore(), false))
	cborStore := cbornode.NewGetManyCborStore(bs)
	cbornode.Cborstr("some data")
	obj1 := cbornode.Cborstr("some data1")
	obj2 := cbornode.Cborstr("some data2")
	obj3 := cbornode.Cborstr("some data3")
	obj4 := cbornode.Cborstr("some data4")
	expectedResults := []interface{}{obj1, obj2, obj4}
	_, err := cborStore.PutMany(bg, expectedResults)
	if err != nil {
		t.Fatal(err)
	}

	outs := []interface{}{new(cbornode.CborByteArray),
		new(cbornode.CborByteArray),
		new(cbornode.CborByteArray),
		new(cbornode.CborByteArray)}
	cursors, missingCIDs, err := cborStore.GetMany(bg, []cid.Cid{obj1.Cid(), obj2.Cid(), obj3.Cid(), obj4.Cid()}, outs)
	if err != nil {
		t.Fatal(err)
	}
	if len(missingCIDs) != 1 {
		println(len(missingCIDs))
		t.Fatal("unexpected number of missing CIDs")
	}
	for cursor := range cursors {
		if !expectedResults[cursor.Index].(*cbornode.CborByteArray).Cid().Equals(cursor.CID) {
			t.Fatalf("expected cid %s does not match actual result %s", expectedResults[cursor.Index].(*cbornode.CborByteArray).Cid().String(), cursor.CID.String())
		}
		if !bytes.Equal(expectedResults[cursor.Index].(*cbornode.CborByteArray).RawData(), outs[cursor.Index].(*cbornode.CborByteArray).RawData()) {
			t.Fatalf("expected result %x does not match actual result %x", expectedResults[cursor.Index].(*cbornode.CborByteArray).RawData(), outs[cursor.Index].(*cbornode.CborByteArray).RawData())
		}
	}
	if !bytes.Equal(missingCIDs[0].Bytes(), obj3.Cid().Bytes()) {
		t.Fail()
	}
}
