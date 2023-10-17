package cbornode

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	block "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	recbor "github.com/polydawn/refmt/cbor"
	atlas "github.com/polydawn/refmt/obj/atlas"
	cbg "github.com/whyrusleeping/cbor-gen"
)

const DefaultMultihash = uint64(mh.BLAKE2B_MIN + 31)

// IpldStore wraps a Blockstore and provides an interface for storing and retrieving CBOR encoded data.
type IpldStore interface {
	Get(ctx context.Context, c cid.Cid, out interface{}) error
	Put(ctx context.Context, v interface{}) (cid.Cid, error)
}

// IpldBlockstore defines a subset of the go-ipfs-blockstore Blockstore interface providing methods
// for storing and retrieving block-centered data.
type IpldBlockstore interface {
	Get(context.Context, cid.Cid) (block.Block, error)
	Put(context.Context, block.Block) error
}

// IpldBlockstoreViewer is a trait that enables zero-copy access to blocks in
// a blockstore.
type IpldBlockstoreViewer interface {
	// View provides zero-copy access to blocks in a blockstore. The callback
	// function will be invoked with the value for the key. The user MUST not
	// modify the byte array, as it could be memory-mapped.
	View(cid.Cid, func([]byte) error) error
}

// BasicIpldStore wraps and IpldBlockstore and implements the IpldStore interface.
type BasicIpldStore struct {
	Blocks IpldBlockstore
	Viewer IpldBlockstoreViewer

	Atlas *atlas.Atlas

	DefaultMultihash uint64
}

var _ IpldStore = &BasicIpldStore{}

// NewCborStore returns an IpldStore implementation backed by the provided IpldBlockstore.
func NewCborStore(bs IpldBlockstore) *BasicIpldStore {
	viewer, _ := bs.(IpldBlockstoreViewer)
	return &BasicIpldStore{Blocks: bs, Viewer: viewer}
}

// Get reads and unmarshals the content at `c` into `out`.
func (s *BasicIpldStore) Get(ctx context.Context, c cid.Cid, out interface{}) error {
	if s.Viewer != nil {
		// zero-copy path.
		return s.Viewer.View(c, func(b []byte) error {
			return s.decode(b, out)
		})
	}

	blk, err := s.Blocks.Get(ctx, c)
	if err != nil {
		return err
	}
	return s.decode(blk.RawData(), out)
}

func (s *BasicIpldStore) decode(b []byte, out interface{}) error {
	cu, ok := out.(cbg.CBORUnmarshaler)
	if ok {
		if err := cu.UnmarshalCBOR(bytes.NewReader(b)); err != nil {
			return NewSerializationError(err)
		}
		return nil
	}

	if s.Atlas == nil {
		return DecodeInto(b, out)
	} else {
		return recbor.UnmarshalAtlased(recbor.DecodeOptions{}, b, out, *s.Atlas)
	}
}

type cidProvider interface {
	Cid() cid.Cid
}

// Put marshals and writes content `v` to the backing blockstore returning its CID.
func (s *BasicIpldStore) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	mhType := DefaultMultihash
	if s.DefaultMultihash != 0 {
		mhType = s.DefaultMultihash
	}

	mhLen := -1
	codec := uint64(cid.DagCBOR)

	var expCid cid.Cid
	if c, ok := v.(cidProvider); ok {
		expCid := c.Cid()
		pref := expCid.Prefix()
		mhType = pref.MhType
		mhLen = pref.MhLength
		codec = pref.Codec
	}

	cm, ok := v.(cbg.CBORMarshaler)
	if ok {
		buf := new(bytes.Buffer)
		if err := cm.MarshalCBOR(buf); err != nil {
			return cid.Undef, NewSerializationError(err)
		}

		pref := cid.Prefix{
			Codec:    codec,
			MhType:   mhType,
			MhLength: mhLen,
			Version:  1,
		}
		c, err := pref.Sum(buf.Bytes())
		if err != nil {
			return cid.Undef, err
		}

		blk, err := block.NewBlockWithCid(buf.Bytes(), c)
		if err != nil {
			return cid.Undef, err
		}

		if err := s.Blocks.Put(ctx, blk); err != nil {
			return cid.Undef, err
		}

		blkCid := blk.Cid()
		if expCid != cid.Undef && blkCid != expCid {
			return cid.Undef, fmt.Errorf("your object is not being serialized the way it expects to")
		}

		return blkCid, nil
	}

	nd, err := WrapObject(v, mhType, mhLen)
	if err != nil {
		return cid.Undef, err
	}

	if err := s.Blocks.Put(ctx, nd); err != nil {
		return cid.Undef, err
	}

	ndCid := nd.Cid()
	if expCid != cid.Undef && ndCid != expCid {
		return cid.Undef, fmt.Errorf("your object is not being serialized the way it expects to")
	}

	return ndCid, nil
}

func NewSerializationError(err error) error {
	return SerializationError{err}
}

type SerializationError struct {
	err error
}

func (se SerializationError) Error() string {
	return se.err.Error()
}

func (se SerializationError) Unwrap() error {
	return se.err
}

func (se SerializationError) Is(o error) bool {
	_, ok := o.(*SerializationError)
	return ok
}

func NewMemCborStore() IpldStore {
	return NewCborStore(newMockBlocks())
}

type mockBlocks struct {
	data map[cid.Cid]block.Block
}

func newMockBlocks() *mockBlocks {
	return &mockBlocks{make(map[cid.Cid]block.Block)}
}

func (mb *mockBlocks) Get(ctx context.Context, c cid.Cid) (block.Block, error) {
	d, ok := mb.data[c]
	if ok {
		return d, nil
	}
	return nil, fmt.Errorf("not found %s", c)
}

func (mb *mockBlocks) Put(ctx context.Context, b block.Block) error {
	mb.data[b.Cid()] = b
	return nil
}

type Cursor struct {
	CID   cid.Cid
	Index int
	Err   error
}

// IpldBatchOpStore wraps a GetManyBlockstore and provides an interface for retrieving CBOR encoded data in batches
type IpldBatchOpStore interface {
	GetMany(ctx context.Context, cs []cid.Cid, outs []interface{}) (<-chan Cursor, []cid.Cid, error)
	PutMany(ctx context.Context, vs []interface{}) ([]cid.Cid, error)
}

// IpldBatchOpBlockStore defines a subset of the go-ipfs-blockstore Blockstore interface providing a method
// for retrieving block-centered data in batches
type IpldBatchOpBlockStore interface {
	IpldBlockstore
	PutMany(ctx context.Context, blocks []block.Block) error
	GetMany(ctx context.Context, cs []cid.Cid) ([]block.Block, []cid.Cid, error)
}

// BatchOpIpldStore wraps and IpldBlockstore and implements the IpldBatchOpStore interface.
type BatchOpIpldStore struct {
	*BasicIpldStore
	BatchOps IpldBatchOpBlockStore
}

var _ IpldStore = &BatchOpIpldStore{}
var _ IpldBatchOpStore = &BatchOpIpldStore{}

// NewGetManyCborStore returns an IpldStore implementation backed by the provided IpldGetManyBlockStore.
func NewGetManyCborStore(bs IpldBatchOpBlockStore) *BatchOpIpldStore {
	viewer, _ := bs.(IpldBlockstoreViewer)
	bis := &BasicIpldStore{Blocks: bs, Viewer: viewer}
	return &BatchOpIpldStore{BatchOps: bs, BasicIpldStore: bis}
}

// GetMany reads and unmarshals the content at `cs` into `outs`
// it returns a channel for tracking the position, identify, and any decode errors in output list
// as well as a list of all the CIDs that could not be retrieved from the underlying blockstore
func (b *BatchOpIpldStore) GetMany(ctx context.Context, cs []cid.Cid, outs []interface{}) (<-chan Cursor, []cid.Cid, error) {
	if len(cs) != len(outs) {
		return nil, nil, errors.New("expected list of cids to be the same length as the destination decode list")
	}
	blks, missingCIDs, err := b.BatchOps.GetMany(ctx, cs)
	if err != nil {
		return nil, nil, err
	}
	cursors := make(chan Cursor)
	go func() {
		for i, blk := range blks {
			err := b.decode(blk.RawData(), outs[i])
			cursors <- Cursor{
				CID:   blk.Cid(),
				Index: i,
				Err:   err,
			}
		}
		close(cursors) // expected behavior is that the Cursor channel will be drained by the receiver even
		// if errors are encountered. If not, and this close is never reached, the goroutine will be eligible for GC
		// once the dropped channel reference is GC'ed
	}()
	return cursors, missingCIDs, nil
}

// PutMany marshals and writes the content in `vs` into the underlying blockstore
func (b *BatchOpIpldStore) PutMany(ctx context.Context, vs []interface{}) ([]cid.Cid, error) {
	mhType := DefaultMultihash
	if b.DefaultMultihash != 0 {
		mhType = b.DefaultMultihash
	}

	mhLen := -1
	codec := uint64(cid.DagCBOR)
	// convert vs to IPLD blocks
	blocks := make([]block.Block, len(vs))
	cids := make([]cid.Cid, len(vs))
	for i, v := range vs {
		var expCid cid.Cid
		if c, ok := v.(cidProvider); ok {
			expCid = c.Cid()
			pref := expCid.Prefix()
			mhType = pref.MhType
			mhLen = pref.MhLength
			codec = pref.Codec
		}

		cm, ok := v.(cbg.CBORMarshaler)
		if ok {
			buf := new(bytes.Buffer)
			if err := cm.MarshalCBOR(buf); err != nil {
				return nil, err
			}

			pref := cid.Prefix{
				Codec:    codec,
				MhType:   mhType,
				MhLength: mhLen,
				Version:  1,
			}
			c, err := pref.Sum(buf.Bytes())
			if err != nil {
				return nil, err
			}

			blk, err := block.NewBlockWithCid(buf.Bytes(), c)
			if err != nil {
				return nil, err
			}

			blkCid := blk.Cid()
			if expCid != cid.Undef && blkCid != expCid {
				return nil, fmt.Errorf("your object is not being serialized the way it expects to\r\n"+
					"expected cid: %s actual cid: %s", expCid.String(), blkCid.String())
			}

			cids[i] = blkCid
			blocks[i] = blk
			continue
		}

		nd, err := WrapObject(v, mhType, mhLen)
		if err != nil {
			return nil, err
		}

		ndCid := nd.Cid()
		if expCid != cid.Undef && ndCid != expCid {
			return nil, fmt.Errorf("your object is not being serialized the way it expects to\r\n"+
				"expected cid: %s actual cid: %s", expCid.String(), ndCid.String())
		}

		cids[i] = ndCid
		blocks[i] = nd
	}

	return cids, b.BatchOps.PutMany(ctx, blocks)
}
