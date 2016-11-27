package cbornode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	cid "github.com/ipfs/go-cid"
	node "github.com/ipfs/go-ipld-node"
	mh "github.com/multiformats/go-multihash"
	cbor "github.com/whyrusleeping/cbor/go"
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

func (n *Node) Copy() node.Node {
	links := make([]*node.Link, len(n.links))
	copy(links, n.links)

	raw := make([]byte, len(n.raw))
	copy(raw, n.raw)

	tree := make([]string, len(n.tree))
	copy(tree, n.tree)

	return &Node{
		obj:   copyObj(n.obj).(map[interface{}]interface{}),
		links: links,
		raw:   raw,
		tree:  tree,
	}
}

func copyObj(i interface{}) interface{} {
	switch i := i.(type) {
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

func (n *Node) compTree() ([]string, error) {
	var out []string
	err := traverse(n.obj, "", func(name string, val interface{}) error {
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

func traverse(obj interface{}, cur string, cb func(string, interface{}) error) error {
	if err := cb(cur, obj); err != nil {
		return err
	}

	switch obj := obj.(type) {
	case map[interface{}]interface{}:
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

func (n Node) RawData() []byte {
	return n.raw
}

func (n *Node) rawData() ([]byte, error) {
	return cbor.Dumps(n.obj)
}

func (n Node) Cid() *cid.Cid {
	data := n.RawData()
	hash, _ := mh.Sum(data, mh.SHA2_256, -1)
	return cid.NewCidV1(cid.DagCBOR, hash)
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

func FromJson(r io.Reader) (*Node, error) {
	var m map[string]interface{}
	err := json.NewDecoder(r).Decode(&m)
	if err != nil {
		return nil, err
	}

	return convertJsonToCbor(m)
}

func convertJsonToCbor(from map[string]interface{}) (*Node, error) {
	out, err := convertMapSIToCbor(from)
	if err != nil {
		return nil, err
	}

	return WrapMap(out)
}

func convertMapSIToCbor(from map[string]interface{}) (map[interface{}]interface{}, error) {
	to := make(map[interface{}]interface{})
	for k, v := range from {
		out, err := convertToCborIshObj(v)
		if err != nil {
			return nil, err
		}
		to[k] = out
	}

	return to, nil
}

func convertToCborIshObj(i interface{}) (interface{}, error) {
	switch v := i.(type) {
	case map[string]interface{}:
		if lnk, ok := v["/"]; ok && len(v) == 1 {
			// special case for links
			vstr, ok := lnk.(string)
			if !ok {
				return nil, fmt.Errorf("link should have been a string")
			}

			c, err := cid.Decode(vstr)
			if err != nil {
				return nil, err
			}

			return &Link{Target: c}, nil
		}

		return convertMapSIToCbor(v)
	case []interface{}:
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

var _ node.Node = (*Node)(nil)
