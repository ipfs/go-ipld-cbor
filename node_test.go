package cbornode

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
	"strings"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"
	mh "github.com/multiformats/go-multihash"
)

func init() {
	RegisterCborType(BigIntAtlasEntry)
}

func assertCid(c cid.Cid, exp string) error {
	if c.String() != exp {
		return fmt.Errorf("expected cid of %s, got %s", exp, c)
	}
	return nil
}

func TestNonObject(t *testing.T) {
	nd, err := WrapObject("", mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertCid(nd.Cid(), "bafyreiengp2sbi6ez34a2jctv34bwyjl7yoliteleaswgcwtqzrhmpyt2m"); err != nil {
		t.Fatal(err)
	}

	back, err := Decode(nd.Copy().RawData(), mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	if err := assertCid(back.Cid(), "bafyreiengp2sbi6ez34a2jctv34bwyjl7yoliteleaswgcwtqzrhmpyt2m"); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeInto(t *testing.T) {
	nd, err := WrapObject(map[string]string{
		"name": "foo",
	}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	err = DecodeInto(nd.RawData(), &m)
	if err != nil {
		t.Fatal(err)
	}

	if len(m) != 1 || m["name"] != "foo" {
		t.Fatal("failed to decode object")
	}
}

func TestDecodeIntoNonObject(t *testing.T) {
	nd, err := WrapObject("foobar", mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	var s string
	err = DecodeInto(nd.RawData(), &s)
	if err != nil {
		t.Fatal(err)
	}
	if s != "foobar" {
		t.Fatal("strings don't match")
	}
}

func TestBasicMarshal(t *testing.T) {
	c := cid.NewCidV0(u.Hash([]byte("something")))

	obj := map[string]interface{}{
		"name": "foo",
		"bar":  c,
	}
	nd, err := WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	if err := assertCid(nd.Cid(), "bafyreib4hmpkwa7zyzoxmpwykof6k7akxnvmsn23oiubsey4e2tf6gqlui"); err != nil {
		t.Fatal(err)
	}

	back, err := Decode(nd.RawData(), mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	if err := assertCid(back.Cid(), "bafyreib4hmpkwa7zyzoxmpwykof6k7akxnvmsn23oiubsey4e2tf6gqlui"); err != nil {
		t.Fatal(err)
	}

	lnk, _, err := back.ResolveLink([]string{"bar"})
	if err != nil {
		t.Fatal(err)
	}

	if !lnk.Cid.Equals(c) {
		t.Fatal("expected cid to match")
	}

	if !nd.Cid().Equals(back.Cid()) {
		t.Fatal("re-serialize failed to generate same cid")
	}
}

func TestMarshalRoundtrip(t *testing.T) {
	c1 := cid.NewCidV0(u.Hash([]byte("something1")))
	c2 := cid.NewCidV0(u.Hash([]byte("something2")))
	c3 := cid.NewCidV0(u.Hash([]byte("something3")))

	obj := map[string]interface{}{
		"foo":   "bar",
		"hello": c1,
		"baz": []interface{}{
			c1,
			c2,
		},
		"cats": map[string]interface{}{
			"qux": c3,
		},
	}

	nd1, err := WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	if err := assertCid(nd1.Cid(), "bafyreibgx4rjaqolj7c32c7ibxc5tedhisc4d23ihx5t4tgamuvy2hvwjm"); err != nil {
		orig, err1 := json.Marshal(obj)
		if err1 != nil {
			t.Fatal(err1)
		}
		js, err1 := nd1.MarshalJSON()
		if err1 != nil {
			t.Fatal(err1)
		}
		t.Fatalf("%s != %s\n%s", orig, js, err)
	}

	if len(nd1.Links()) != 4 {
		t.Fatal("didnt have enough links")
	}

	nd2, err := Decode(nd1.RawData(), mh.SHA2_256, -1)
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

func assertStringsEqual(t *testing.T, a, b []string) {
	if len(a) != len(b) {
		t.Fatal("lengths differed: ", a, b)
	}

	sort.Strings(a)
	sort.Strings(b)

	for i, v := range a {
		if v != b[i] {
			t.Fatal("got mismatch: ", a, b)
		}
	}
}

func TestTree(t *testing.T) {
	c1 := cid.NewCidV0(u.Hash([]byte("something1")))
	c2 := cid.NewCidV0(u.Hash([]byte("something2")))
	c3 := cid.NewCidV0(u.Hash([]byte("something3")))
	c4 := cid.NewCidV0(u.Hash([]byte("something4")))

	obj := map[string]interface{}{
		"foo": c1,
		"baz": []interface{}{c2, c3, "c"},
		"cats": map[string]interface{}{
			"qux": map[string]interface{}{
				"boo": 1,
				"baa": c4,
				"bee": 3,
				"bii": 4,
				"buu": map[string]string{
					"coat": "rain",
				},
			},
		},
	}

	nd, err := WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertCid(nd.Cid(), "bafyreicp66zmx7grdrnweetu23anx3e5zguda7646iwyothju6nhgqykgq"); err != nil {
		t.Fatal(err)
	}

	full := []string{
		"foo",
		"baz",
		"baz/0",
		"baz/1",
		"baz/2",
		"cats",
		"cats/qux",
		"cats/qux/boo",
		"cats/qux/baa",
		"cats/qux/bee",
		"cats/qux/bii",
		"cats/qux/buu",
		"cats/qux/buu/coat",
	}

	assertStringsEqual(t, full, nd.Tree("", -1))

	cats := []string{
		"qux",
		"qux/boo",
		"qux/baa",
		"qux/bee",
		"qux/bii",
		"qux/buu",
		"qux/buu/coat",
	}

	assertStringsEqual(t, cats, nd.Tree("cats", -1))

	toplevel := []string{
		"foo",
		"baz",
		"cats",
	}

	assertStringsEqual(t, toplevel, nd.Tree("", 1))
	assertStringsEqual(t, []string{}, nd.Tree("", 0))
}

func TestParsing(t *testing.T) {
	// This shouldn't pass
	// Debug representation from cbor.io is
	//
	// D9 0102                              # tag(258)
	// 58 25                                # bytes(37)
	//    A503221220659650FC3443C916428048EFC5BA4558DC863594980A59F5CB3C4D84867E6D31 # "\xA5\x03\"\x12 e\x96P\xFC4C\xC9\x16B\x80H\xEF\xC5\xBAEX\xDC\x865\x94\x98\nY\xF5\xCB<M\x84\x86~m1"
	//
	t.Skip()

	b := []byte("\xd9\x01\x02\x58\x25\xa5\x03\x22\x12\x20\x65\x96\x50\xfc\x34\x43\xc9\x16\x42\x80\x48\xef\xc5\xba\x45\x58\xdc\x86\x35\x94\x98\x0a\x59\xf5\xcb\x3c\x4d\x84\x86\x7e\x6d\x31")

	n, err := Decode(b, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	if err := assertCid(n.Cid(), "bafyreib4jqzv5rpohiu2vi56uggpnshhhnqrsclx2mji67wnihkrfboox4"); err != nil {
		t.Fatal(err)
	}

	n, err = Decode(b, mh.SHA2_512, -1)
	if err != nil {
		t.Fatal(err)
	}

	if err := assertCid(n.Cid(), "bafyrgqcomxa52fzyrx6d46sgposgxrzdyqpbtu5adh7fpjjhqo7mwftoexd26ghhhxcq53hczq3dobudmvyegjhamahzdu2k66jklr5uys3tq"); err != nil {
		t.Fatal(err)
	}
}

func TestFromJson(t *testing.T) {
	data := `{
        "something": {"/":"bafkreifvxooyaffa7gy5mhrb46lnpdom34jvf4r42mubf5efbodyvzeujq"},
        "cats": "not cats",
        "cheese": [
                {"/":"bafkreifvxooyaffa7gy5mhrb46lnpdom34jvf4r42mubf5efbodyvzeujq"},
                {"/":"bafkreifvxooyaffa7gy5mhrb46lnpdom34jvf4r42mubf5efbodyvzeujq"},
                {"/":"bafkreifvxooyaffa7gy5mhrb46lnpdom34jvf4r42mubf5efbodyvzeujq"},
                {"/":"bafkreifvxooyaffa7gy5mhrb46lnpdom34jvf4r42mubf5efbodyvzeujq"}
        ]
}`
	n, err := FromJSON(bytes.NewReader([]byte(data)), mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertCid(n.Cid(), "bafyreicnokmhmrnlp2wjhyk2haep4tqxiptwfrp2rrs7rzq7uk766chqvq"); err != nil {
		t.Fatal(err)
	}

	c, ok := n.obj.(map[string]interface{})["something"].(cid.Cid)
	if !ok {
		fmt.Printf("%#v\n", n.obj)
		t.Fatal("expected a cid")
	}

	if c.String() != "bafkreifvxooyaffa7gy5mhrb46lnpdom34jvf4r42mubf5efbodyvzeujq" {
		t.Fatal("cid unmarshaled wrong")
	}
}

func TestResolvedValIsJsonable(t *testing.T) {
	data := `{
		"foo": {
			"bar": 1,
			"baz": 2
		}
	}`
	n, err := FromJSON(strings.NewReader(data), mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertCid(n.Cid(), "bafyreiahcy6ewqmabbh7lcjhxrillpf72zlu3vqcovckanvj2fwdtenvbe"); err != nil {
		t.Fatal(err)
	}

	val, _, err := n.Resolve([]string{"foo"})
	if err != nil {
		t.Fatal(err)
	}

	out, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != `{"bar":1,"baz":2}` {
		t.Fatal("failed to get expected json")
	}
}

func TestExamples(t *testing.T) {
	examples := map[string]string{
		"[null]":                        "bafyreigtpkiih7wr7wb7ts6j5aunnotxiff3yqkx33rs4k4xhskprx5tui",
		"[]":                            "bafyreidwx2fvfdiaox32v2mnn6sxu3j4qoxeqcuenhtgrv5qv6litfnmoe",
		"{}":                            "bafyreigbtj4x7ip5legnfznufuopl4sg4knzc2cof6duas4b3q2fy6swua",
		"null":                          "bafyreifqwkmiw256ojf2zws6tzjeonw6bpd5vza4i22ccpcq4hjv2ts7cm",
		"1":                             "bafyreihtx752fmf3zafbys5dtr4jxohb53yi3qtzfzf6wd5274jwtn5agu",
		"[1]":                           "bafyreihwrdqkjomfjaoqe5hbpfjzqoxkhptohvoa5u362s6obgpvxcw45q",
		"true":                          "bafyreibhvppn37ufanewvxvwendgzksh3jpwhk6sxrx2dh3m7s3t5t7noa",
		`{"a":"IPFS"}`:                  "bafyreihyyz2badz34h5pvcgof4fj3qwwr7mopoucejwbnpzs7soorkrct4",
		`{"a":"IPFS","b":null,"c":[1]}`: "bafyreigg2gcszayx2lywb3edqfoftyvus7gxeanmudqla3e6eh2okei25a",
		`{"a":[]}`:                      "bafyreian4t6wau4jdqt6nys76dfvsn6g7an4ulbv326yzutdgnrr5cjpui",
	}
	for originalJSON, expcid := range examples {
		t.Run(originalJSON, func(t *testing.T) {
			check := func(err error) {
				if err != nil {
					t.Fatalf("for object %s: %s", originalJSON, err)
				}
			}

			n, err := FromJSON(bytes.NewReader([]byte(originalJSON)), mh.SHA2_256, -1)
			check(err)
			check(assertCid(n.Cid(), expcid))

			cbor := n.RawData()
			_, err = Decode(cbor, mh.SHA2_256, -1)
			check(err)

			node, err := Decode(cbor, mh.SHA2_256, -1)
			check(err)

			jsonBytes, err := node.MarshalJSON()
			check(err)

			json := string(jsonBytes)
			if json != originalJSON {
				t.Fatalf("marshaled to incorrect JSON: %s != %s", originalJSON, json)
			}
		})
	}
}

func TestObjects(t *testing.T) {
	raw, err := os.ReadFile("test_objects/expected.json")
	if err != nil {
		t.Fatal(err)
	}

	var cases map[string]map[string]string
	err = json.Unmarshal(raw, &cases)
	if err != nil {
		t.Fatal(err)
	}

	for k, c := range cases {
		t.Run(k, func(t *testing.T) {
			in, err := os.ReadFile(fmt.Sprintf("test_objects/%s.json", k))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(fmt.Sprintf("test_objects/%s.cbor", k))
			if err != nil {
				t.Fatal(err)
			}

			nd, err := FromJSON(bytes.NewReader(in), mh.SHA2_256, -1)
			if err != nil {
				t.Fatal(err)
			}

			cExp, err := cid.Decode(c["/"])
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(nd.RawData(), expected) {
				t.Fatalf("bytes do not match: %x != %x", nd.RawData(), expected)
			}

			if !nd.Cid().Equals(cExp) {
				t.Fatalf("cid missmatch: %s != %s", nd.String(), cExp.String())
			}
		})
	}
}

func TestCanonicalize(t *testing.T) {
	b, err := os.ReadFile("test_objects/non-canon.cbor")
	if err != nil {
		t.Fatal(err)
	}
	nd1, err := Decode(b, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(b, nd1.RawData()) {
		t.Fatal("failed to canonicalize node")
	}

	if err := assertCid(nd1.Cid(), "bafyreiawx7ona7oa2ptcoh6vwq4q6bmd7x2ibtkykld327bgb7t73ayrqm"); err != nil {
		t.Fatal(err)
	}

	nd2, err := Decode(nd1.RawData(), mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	if !nd2.Cid().Equals(nd1.Cid()) || !bytes.Equal(nd2.RawData(), nd1.RawData()) {
		t.Fatal("re-decoding a canonical node should be idempotent")
	}
}

func TestStableCID(t *testing.T) {
	b, err := os.ReadFile("test_objects/non-canon.cbor")
	if err != nil {
		t.Fatal(err)
	}

	hash, err := mh.Sum(b, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	c := cid.NewCidV1(cid.DagCBOR, hash)

	badBlock, err := blocks.NewBlockWithCid(b, c)
	if err != nil {
		t.Fatal(err)
	}
	badNode, err := DecodeBlock(badBlock)
	if err != nil {
		t.Fatal(err)
	}

	if !badBlock.Cid().Equals(badNode.Cid()) {
		t.Fatal("CIDs not stable")
	}
}

func TestCidAndBigInt(t *testing.T) {
	type Foo struct {
		B *big.Int
		A cid.Cid
	}
	RegisterCborType(Foo{})

	nd, err := WrapObject("", mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	c := nd.Cid()
	_, err = WrapObject(&Foo{
		A: c,
		B: big.NewInt(1),
	}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEmptyCid(t *testing.T) {
	type Foo struct {
		A cid.Cid
	}
	type Bar struct {
		A cid.Cid `refmt:",omitempty"`
	}
	RegisterCborType(Foo{})
	RegisterCborType(Bar{})

	_, err := WrapObject(&Foo{}, mh.SHA2_256, -1)
	if err == nil {
		t.Fatal("should have failed to encode an object with an empty but non-omitted CID")
	}

	_, err = WrapObject(&Bar{}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalStructEncoding(t *testing.T) {
	type Foo struct {
		Zebra string
		Dog   int
		Cats  float64
		Whale string
		Cat   bool
	}
	RegisterCborType(Foo{})

	s := Foo{
		Zebra: "seven",
		Dog:   15,
		Cats:  1.519,
		Whale: "never",
		Cat:   true,
	}

	m, err := WrapObject(s, math.MaxUint64, -1)
	if err != nil {
		t.Fatal(err)
	}

	expraw, err := hex.DecodeString("a563636174f563646f670f6463617473fb3ff84dd2f1a9fbe7657768616c65656e65766572657a6562726165736576656e")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(expraw, m.RawData()) {
		t.Fatal("not canonical")
	}
}

type TestMe struct {
	Hello *big.Int
	World big.Int
	Hi    int
}

func TestBigIntRoundtrip(t *testing.T) {
	RegisterCborType(TestMe{})

	one := TestMe{
		Hello: big.NewInt(100),
		World: *big.NewInt(99),
	}

	bytes, err := DumpObject(&one)
	if err != nil {
		t.Fatal(err)
	}

	var oneBack TestMe
	if err := DecodeInto(bytes, &oneBack); err != nil {
		t.Fatal(err)
	}

	if one.Hello.Cmp(oneBack.Hello) != 0 {
		t.Fatal("failed to roundtrip *big.Int")
	}

	if one.World.Cmp(&oneBack.World) != 0 {
		t.Fatal("failed to roundtrip big.Int")
	}

	list := map[string]*TestMe{
		"hello": {Hello: big.NewInt(10), World: *big.NewInt(101), Hi: 1},
		"world": {Hello: big.NewInt(9), World: *big.NewInt(901), Hi: 3},
	}

	bytes, err = DumpObject(list)
	if err != nil {
		t.Fatal(err)
	}

	var listBack map[string]*TestMe
	if err := DecodeInto(bytes, &listBack); err != nil {
		t.Fatal(err)
	}

	t.Log(listBack["hello"])
	t.Log(listBack["world"])

	if list["hello"].Hello.Cmp(listBack["hello"].Hello) != 0 {
		t.Fatalf("failed to roundtrip *big.Int: %s != %s", list["hello"].Hello, listBack["hello"].Hello)
	}

	if list["hello"].World.Cmp(&listBack["hello"].World) != 0 {
		t.Fatalf("failed to roundtrip big.Int: %s != %s", &list["hello"].World, &listBack["hello"].World)
	}

	if list["world"].Hello.Cmp(listBack["world"].Hello) != 0 {
		t.Fatalf("failed to roundtrip *big.Int: %s != %s", list["world"].Hello, listBack["world"].Hello)
	}

	if list["world"].World.Cmp(&listBack["world"].World) != 0 {
		t.Fatalf("failed to roundtrip big.Int: %s != %s", &list["world"].World, &listBack["world"].World)
	}

}
