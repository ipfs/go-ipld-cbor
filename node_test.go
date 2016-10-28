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

func TestMarshalRoundtrip(t *testing.T) {
	c1 := cid.NewCidV0(u.Hash([]byte("something1")))
	c2 := cid.NewCidV0(u.Hash([]byte("something2")))
	c3 := cid.NewCidV0(u.Hash([]byte("something3")))

	obj := map[interface{}]interface{}{
		"foo": "bar",
		"baz": []interface{}{
			&Link{c1},
			&Link{c2},
		},
		"cats": map[interface{}]interface{}{
			"qux": &Link{c3},
		},
	}

	nd1, err := WrapMap(obj)
	if err != nil {
		t.Fatal(err)
	}

	nd2, err := Decode(nd1.RawData())
	if err != nil {
		t.Fatal(err)
	}

	if !nd1.Cid().Equals(nd2.Cid()) {
		t.Fatal("objects didnt match between marshalings")
	}

	lnk, rest, err := nd2.ResolveLink([]string{"baz", "1", "bop"})
	if err != nil {
		t.Fatal(err)
	}

	if !lnk.Cid.Equals(c2) {
		t.Fatal("expected c2")
	}

	if len(rest) != 1 || rest[0] != "bop" {
		t.Fatal("should have had one path element remaning after resolve")
	}

	out, err := nd1.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(out))
}
