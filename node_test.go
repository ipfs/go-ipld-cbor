package cbornode

import (
	"testing"

	cid "github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"
	cbor "github.com/whyrusleeping/cbor/go"
)

type testObject struct {
	Name string
	Bar  *Link
}

func TestBasicMarshal(t *testing.T) {
	c := cid.NewCidV0(u.Hash([]byte("something")))

	obj := testObject{
		Name: "foo",
		Bar:  &Link{c},
	}

	out, err := cbor.Dumps(&obj)
	if err != nil {
		t.Fatal(err)
	}

	back, err := Decode(out)
	if err != nil {
		t.Fatal(err)
	}

	lnk, _, err := back.ResolveLink([]string{"Bar"})
	if err != nil {
		t.Fatal(err)
	}

	if !lnk.Cid.Equals(c) {
		t.Fatal("expected cid to match")
	}

	var obj2 testObject
	err = DecodeInto(out, &obj2)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%#v", obj2)
}
