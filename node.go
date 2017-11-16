package cbornode

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	node "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"

	cbor "github.com/polydawn/refmt/cbor"
	"github.com/polydawn/refmt/obj/atlas"
)

// CBORTagLink is the integer used to represent tags in CBOR.
const CBORTagLink = 42

// Node represents an IPLD node.
type Node struct {
	obj   interface{}
	tree  []string
	links []*node.Link
	raw   []byte
	cid   cid.Cid
}

// ErrNoSuchLink is returned when no link with the given name was found.
var ErrNoSuchLink = errors.New("no such link found")

var cborAtlas = atlas.MustBuild(
	atlas.
		BuildEntry(cid.Cid{}).
		UseTag(CBORTagLink).
		Transform().
		TransformMarshal(atlas.MakeMarshalTransformFunc(
			func(link cid.Cid) ([]byte, error) {
				// TODO: manually doing binary multibase
				return castCidToBytes(link), nil
			})).
		TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
			func(x []byte) (cid.Cid, error) {
				c, err := castBytesToCid(x)
				if err != nil {
					return cid.Cid{}, err
				}

				return c, nil
			})).
		Complete(),
)

// DecodeBlock decodes a CBOR encoded Block into an IPLD Node.
//
// This method *does not* canonicalize and *will* preserve the CID. As a matter
// of fact, it will assume that `block.Cid()` returns the correct CID and will
// make no effort to validate this assumption.
//
// In general, you should not be calling this method directly. Instead, you
// should be calling the `Decode` method from the `go-ipld-format` package. That
// method will pick the right decoder based on the Block's CID.
//
// Note: This function keeps a reference to `block` and assumes that it is
// immutable.
func DecodeBlock(block blocks.Block) (node.Node, error) {
	return decodeBlock(block)
}

func decodeBlock(block blocks.Block) (*Node, error) {
	m, err := decodeCBOR(block.RawData())
	if err != nil {
		return nil, err
	}
	tree, err := compTree(m)
	if err != nil {
		return nil, err
	}
	links, err := compLinks(m)
	if err != nil {
		return nil, err
	}

	return &Node{
		obj:   m,
		tree:  tree,
		links: links,
		raw:   block.RawData(),
		cid:   block.Cid(),
	}, nil
}

var _ node.DecodeBlockFunc = DecodeBlock

// Decode decodes a CBOR object into an IPLD Node.
//
// If passed a non-canonical CBOR node, this function will canonicalize it.
// Therefore, `bytes.Equal(b, Decode(b).RawData())` may not hold. If you already
// have a CID for this data and want to ensure that it doesn't change, you
// should use `DecodeBlock`.
// mhType is multihash code to use for hashing, for example mh.SHA2_256
//
// Note: This function does not hold onto `b`. You may reuse it.
func Decode(b []byte, mhType uint64, mhLen int) (*Node, error) {
	m, err := decodeCBOR(b)
	if err != nil {
		return nil, err
	}

	// We throw away `b` here to ensure that we canonicalize the encoded
	// CBOR object.
	return WrapObject(m, mhType, mhLen)
}

// DecodeInto decodes a serialized IPLD cbor object into the given object.
func DecodeInto(b []byte, v interface{}) error {
	// The cbor library really doesnt make this sort of operation easy on us
	m, err := decodeCBOR(b)
	if err != nil {
		return err
	}

	jsonable, err := convertToJSONIsh(m)
	if err != nil {
		return err
	}

	jsonb, err := json.Marshal(jsonable)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonb, v)

}

// Decodes a cbor node into an object.
func decodeCBOR(b []byte) (m interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to unmarshal - cbor panic: %s", r)
		}
	}()

	err = cbor.UnmarshalAtlased(b, &m, cborAtlas)
	if err != nil {
		fmt.Printf("bytes: %s\n", hex.EncodeToString(b))
		err = fmt.Errorf("failed to unmarshal: %s", err)
	}
	return
}

// WrapObject converts an arbitrary object into a Node.
func WrapObject(m interface{}, mhType uint64, mhLen int) (*Node, error) {
	fmt.Printf("wrapping object: %v\n", m)
	data, err := DumpObject(m)
	if err != nil {
		return nil, err
	}
	if mhType == math.MaxUint64 {
		mhType = mh.SHA2_256
	}

	hash, err := mh.Sum(data, mhType, mhLen)
	if err != nil {
		return nil, err
	}
	c := cid.NewCidV1(cid.DagCBOR, hash)

	block, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		// TODO: Shouldn't this just panic?
		return nil, err
	}
	// Do not reuse `m`. We need to re-decode it to put it in the right
	// form.
	return decodeBlock(block)
}

// Resolve resolves a given path, and returns the object found at the end, as well
// as the possible tail of the path that was not resolved.
func (n *Node) Resolve(path []string) (interface{}, []string, error) {
	var cur interface{} = n.obj
	for i, val := range path {
		fmt.Printf("cur: %s - %v\n%s\n", reflect.TypeOf(cur), cur, val)

		switch curv := cur.(type) {
		case map[string]interface{}:
			next, ok := curv[val]
			if !ok {
				return nil, nil, ErrNoSuchLink
			}

			cur = next
		case map[interface{}]interface{}:
			next, ok := curv[val]
			if !ok {
				return nil, nil, ErrNoSuchLink
			}

			cur = next
		case []interface{}:
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, nil, err
			}

			if n < 0 || n >= len(curv) {
				return nil, nil, fmt.Errorf("array index out of range")
			}

			cur = curv[n]
		case cid.Cid:
			return &node.Link{Cid: curv}, path[i:], nil
		default:
			return nil, nil, errors.New("tried to resolve through object that had no links")
		}
	}

	lnk, ok := cur.(cid.Cid)
	if ok {
		return &node.Link{Cid: lnk}, nil, nil
	}

	jsonish, err := convertToJSONIsh(cur)
	if err != nil {
		return nil, nil, err
	}

	return jsonish, nil, nil
}

// Copy creates a copy of the Node.
func (n *Node) Copy() node.Node {
	links := make([]*node.Link, len(n.links))
	copy(links, n.links)

	raw := make([]byte, len(n.raw))
	copy(raw, n.raw)

	tree := make([]string, len(n.tree))
	copy(tree, n.tree)

	return &Node{
		obj:   copyObj(n.obj),
		links: links,
		raw:   raw,
		tree:  tree,
		cid:   n.cid,
	}
}

func copyObj(i interface{}) interface{} {
	switch i := i.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{})
		for k, v := range i {
			out[k] = copyObj(v)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[interface{}]interface{})
		for k, v := range i {
			out[k] = copyObj(v)
		}
		return out
	case []interface{}:
		var out []interface{}
		for _, v := range i {
			out = append(out, copyObj(v))
		}
		return out
	default:
		// being lazy for now
		// use caution
		return i
	}
}

// ResolveLink resolves a path and returns the raw Link at the end, as well as
// the possible tail of the path that was not resolved.
func (n *Node) ResolveLink(path []string) (*node.Link, []string, error) {
	obj, rest, err := n.Resolve(path)
	if err != nil {
		return nil, nil, err
	}

	lnk, ok := obj.(*node.Link)
	if !ok {
		fmt.Printf("lnk %v - %s\n", obj, reflect.TypeOf(obj))
		return nil, rest, fmt.Errorf("found non-link at given path")
	}

	return lnk, rest, nil
}

// Tree returns a flattend array of paths at the given path for the given depth.
func (n *Node) Tree(path string, depth int) []string {
	if path == "" && depth == -1 {
		return n.tree
	}

	var out []string
	for _, t := range n.tree {
		if !strings.HasPrefix(t, path) {
			continue
		}

		sub := strings.TrimLeft(t[len(path):], "/")
		if sub == "" {
			continue
		}

		if depth < 0 {
			out = append(out, sub)
			continue
		}

		parts := strings.Split(sub, "/")
		if len(parts) <= depth {
			out = append(out, sub)
		}
	}
	return out
}

func compTree(obj interface{}) ([]string, error) {
	var out []string
	err := traverse(obj, "", func(name string, val interface{}) error {
		if name != "" {
			out = append(out, name[1:])
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// Links lists all known links of the Node.
func (n *Node) Links() []*node.Link {
	return n.links
}

func compLinks(obj interface{}) ([]*node.Link, error) {
	var out []*node.Link
	fmt.Printf("traversing: %v\n", obj)
	err := traverse(obj, "", func(name string, val interface{}) error {
		fmt.Printf("t: %v, %s\n", val, reflect.TypeOf(val))
		if lnk, ok := val.(cid.Cid); ok {
			out = append(out, &node.Link{Cid: lnk})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	fmt.Printf("found links: %v\n", out)
	return out, nil
}

func traverse(obj interface{}, cur string, cb func(string, interface{}) error) error {
	if err := cb(cur, obj); err != nil {
		return err
	}
	fmt.Printf("obj-type: %s\n", reflect.TypeOf(obj))
	switch obj := obj.(type) {
	case map[string]interface{}:
		for k, v := range obj {
			this := cur + "/" + k
			if err := traverse(v, this, cb); err != nil {
				return err
			}
		}
		return nil
	case map[interface{}]interface{}:
		for k, v := range obj {
			ks, ok := k.(string)
			if !ok {
				return errors.New("map key was not a string")
			}
			this := cur + "/" + ks
			if err := traverse(v, this, cb); err != nil {
				return err
			}
		}
		return nil
	case []interface{}:
		for i, v := range obj {
			this := fmt.Sprintf("%s/%d", cur, i)
			if err := traverse(v, this, cb); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

// RawData returns the raw bytes that represent the Node as serialized CBOR.
func (n *Node) RawData() []byte {
	return n.raw
}

// Cid returns the canonical Cid of the NOde.
func (n *Node) Cid() cid.Cid {
	return n.cid
}

// Loggable returns a loggable representation of the Node.
func (n *Node) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"node_type": "cbor",
		"cid":       n.Cid(),
	}
}

// Size returns the size of the binary representation of the Node.
func (n *Node) Size() (uint64, error) {
	return uint64(len(n.RawData())), nil
}

// Stat returns stats about the Node.
// TODO: implement?
func (n *Node) Stat() (*node.NodeStat, error) {
	return &node.NodeStat{}, nil
}

// String returns the string representation of the CID of the Node.
func (n *Node) String() string {
	return n.Cid().String()
}

// MarshalJSON converts the Node into its JSON representation.
func (n *Node) MarshalJSON() ([]byte, error) {
	out, err := convertToJSONIsh(n.obj)
	if err != nil {
		return nil, err
	}

	return json.Marshal(out)
}

// DumpObject marshals any object into its CBOR serialized byte representation
// TODO: rename
func DumpObject(obj interface{}) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to marshal - cbor panic: %s", r)
		}
	}()

	out, err = cbor.MarshalAtlased(obj, cborAtlas)
	if err != nil {
		err = fmt.Errorf("failed to marshal: %s", err)
	}
	return
}

func toSaneMap(n map[interface{}]interface{}) (interface{}, error) {
	if lnk, ok := n["/"]; ok && len(n) == 1 {
		lnkb, ok := lnk.([]byte)
		if !ok {
			return nil, fmt.Errorf("link value should have been bytes")
		}

		c, err := cid.Cast(lnkb)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{"/": c}, nil
	}
	out := make(map[string]interface{})
	for k, v := range n {
		ks, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("map keys must be strings")
		}

		obj, err := convertToJSONIsh(v)
		if err != nil {
			return nil, err
		}

		out[ks] = obj
	}

	return out, nil
}

func convertToJSONIsh(v interface{}) (interface{}, error) {
	switch v := v.(type) {
	case map[interface{}]interface{}:
		return toSaneMap(v)
	case []interface{}:
		var out []interface{}
		if len(v) == 0 && v != nil {
			return []interface{}{}, nil
		}
		for _, i := range v {
			obj, err := convertToJSONIsh(i)
			if err != nil {
				return nil, err
			}

			out = append(out, obj)
		}
		return out, nil
	default:
		return v, nil
	}
}

// FromJSON converts incoming JSON into a Node.
func FromJSON(r io.Reader, mhType uint64, mhLen int) (*Node, error) {
	var m interface{}
	err := json.NewDecoder(r).Decode(&m)
	if err != nil {
		return nil, err
	}

	obj, err := convertToCborIshObj(m)
	if err != nil {
		return nil, err
	}

	fmt.Printf("fromjson: %s - %s\n", reflect.TypeOf(obj), obj)
	return WrapObject(obj, mhType, mhLen)
}

func convertToCborIshObj(i interface{}) (interface{}, error) {
	switch v := i.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			return v, nil
		}

		if lnk, ok := v["/"]; ok && len(v) == 1 {
			// special case for links
			vstr, ok := lnk.(string)
			if !ok {
				return nil, fmt.Errorf("link should have been a string")
			}

			return cid.Decode(vstr)
		}

		return v, nil
	case []interface{}:
		if len(v) == 0 {
			return v, nil
		}

		var out []interface{}
		for _, o := range v {
			obj, err := convertToCborIshObj(o)
			if err != nil {
				return nil, err
			}

			out = append(out, obj)
		}

		return out, nil
	default:
		return v, nil
	}
}

func castBytesToCid(x []byte) (cid.Cid, error) {
	if len(x) == 0 {
		return cid.Cid{}, fmt.Errorf("link value was empty")
	}

	// TODO: manually doing multibase checking here since our deps don't
	// support binary multibase yet
	if x[0] != 0 {
		return cid.Cid{}, fmt.Errorf("invalid multibase on IPLD link")
	}

	c, err := cid.Cast(x[1:])
	if err != nil {
		return cid.Cid{}, fmt.Errorf("invalid IPLD link: %s", err)
	}

	fmt.Printf("decoded cid: %s\n", c.String())
	return c, nil
}

func castCidToBytes(link cid.Cid) []byte {
	return append([]byte{0}, link.Bytes()...)
}

var _ node.Node = (*Node)(nil)
