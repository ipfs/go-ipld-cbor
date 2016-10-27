package cbornode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	cbor "gx/ipfs/QmPL3RCWaM6s7b82LSLS1MGX2jpxPxA1v2vmgLm15b1NcW/cbor/go"
	node "gx/ipfs/QmU7bFWQ793qmvNy7outdCaMfSDNk8uqhx4VNrxYj5fj5g/go-ipld-node"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
)

func Decode(b []byte) (*Node, error) {
	var m map[interface{}]interface{}
	err := cbor.Loads(b, &m)
	if err != nil {
		return nil, err
	}

	return WrapMap(m)
}

// DecodeInto decodes a serialized ipld cbor object into the given object.
func DecodeInto(b []byte, v interface{}) error {
	// The cbor library really doesnt make this sort of operation easy on us when we are implementing
	// the `ToCBOR` method.
	var m map[interface{}]interface{}
	err := cbor.Loads(b, &m)
	if err != nil {
		return err
	}

	jsonable, err := toSaneMap(m)
	if err != nil {
		return err
	}

	jsonb, err := json.Marshal(jsonable)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonb, v)
}

var ErrNoSuchLink = errors.New("no such link found")

type Node struct {
	obj   map[interface{}]interface{}
	tree  []string
	links []*node.Link
	raw   []byte
}

func WrapMap(m map[interface{}]interface{}) (*Node, error) {
	nd := &Node{obj: m}
	tree, err := nd.compTree()
	if err != nil {
		return nil, err
	}

	nd.tree = tree

	links, err := nd.compLinks()
	if err != nil {
		return nil, err
	}

	nd.links = links

	data, err := nd.rawData()
	if err != nil {
		return nil, err
	}

	nd.raw = data

	return nd, nil
}

type Link struct {
	Target *cid.Cid `json:"/" cbor:"/"`
}

func (l *Link) ToCBOR(w io.Writer, enc *cbor.Encoder) error {
	obj := map[string]interface{}{
		"/": l.Target.Bytes(),
	}

	return enc.Encode(obj)
}

func (n Node) Resolve(path []string) (interface{}, []string, error) {
	var cur interface{} = n.obj
	for i, val := range path {
		lnk, err := maybeLink(cur)
		if err != nil {
			return nil, nil, err
		}
		if lnk != nil {
			return lnk, path[i:], nil
		}

		switch curv := cur.(type) {
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
		default:
			return nil, nil, errors.New("tried to resolve through object that had no links")
		}
	}

	lnk, err := maybeLink(cur)
	if err != nil {
		return nil, nil, err
	}
	if lnk != nil {
		return lnk, nil, nil
	}

	return nil, nil, errors.New("could not resolve through object")
}

func maybeLink(i interface{}) (*node.Link, error) {
	m, ok := i.(map[interface{}]interface{})
	if !ok {
		return nil, nil
	}

	lnk, ok := m["/"]
	if !ok {
		return nil, nil
	}

	return linkCast(lnk)
}

func (n Node) ResolveLink(path []string) (*node.Link, []string, error) {
	obj, rest, err := n.Resolve(path)
	if err != nil {
		return nil, nil, err
	}

	lnk, ok := obj.(*node.Link)
	if ok {
		return lnk, rest, nil
	}

	return nil, rest, fmt.Errorf("found non-link at given path")
}

func linkCast(lnk interface{}) (*node.Link, error) {
	lnkb, ok := lnk.([]byte)
	if !ok {
		return nil, errors.New("incorrectly formatted link")
	}

	c, err := cid.Cast(lnkb)
	if err != nil {
		return nil, err
	}

	return &node.Link{Cid: c}, nil
}

func (n *Node) Tree() []string {
	return n.tree
}

func (n *Node) compTree() ([]string, error) {
	var out []string
	err := traverse(n.obj, "", func(name string, val interface{}) error {
		out = append(out, name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (n Node) Links() []*node.Link {
	return n.links
}

func (n *Node) compLinks() ([]*node.Link, error) {
	var out []*node.Link
	err := traverse(n.obj, "", func(_ string, val interface{}) error {
		if lnk, ok := val.(*node.Link); ok {
			out = append(out, lnk)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func traverse(obj map[interface{}]interface{}, cur string, cb func(string, interface{}) error) error {
	if lnk, ok := obj["/"]; ok {
		l, err := linkCast(lnk)
		if err != nil {
			return err
		}

		return cb(cur, l)
	}

	for k, v := range obj {
		ks, ok := k.(string)
		if !ok {
			return errors.New("map key was not a string")
		}
		this := cur + "/" + ks
		switch v := v.(type) {
		case map[interface{}]interface{}:
			if err := traverse(v, this, cb); err != nil {
				return err
			}
		default:
			if err := cb(this, v); err != nil {
				return err
			}
		}
	}

	return nil
}

func (n Node) RawData() []byte {
	return n.raw
}

func (n *Node) rawData() ([]byte, error) {
	return cbor.Dumps(n.obj)
}

func (n Node) Cid() *cid.Cid {
	data := n.RawData()
	hash, _ := mh.Sum(data, mh.SHA2_256, -1)
	return cid.NewCidV1(cid.CBOR, hash)
}

func (n Node) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"node_type": "cbor",
		"cid":       n.Cid(),
	}
}

func (n Node) Size() (uint64, error) {
	return uint64(len(n.RawData())), nil
}

func (n Node) Stat() (*node.NodeStat, error) {
	return &node.NodeStat{}, nil
}

func (n Node) String() string {
	return n.Cid().String()
}

func (n Node) MarshalJSON() ([]byte, error) {
	out, err := toSaneMap(n.obj)
	if err != nil {
		return nil, err
	}

	return json.Marshal(out)
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

		return &Link{c}, nil
	}
	out := make(map[string]interface{})
	for k, v := range n {
		ks, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("map keys must be strings")
		}

		obj, err := convertToJsonIsh(v)
		if err != nil {
			return nil, err
		}

		out[ks] = obj
	}

	return out, nil
}

func convertToJsonIsh(v interface{}) (interface{}, error) {
	switch v := v.(type) {
	case map[interface{}]interface{}:
		return toSaneMap(v)
	case []interface{}:
		var out []interface{}
		for _, i := range v {
			obj, err := convertToJsonIsh(i)
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

var _ node.Node = (*Node)(nil)
