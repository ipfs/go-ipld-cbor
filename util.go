package cbornode

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	cbg "github.com/whyrusleeping/cbor-gen"
)

type CborByteArray []byte

func (c *CborByteArray) MarshalCBOR(w io.Writer) error {
	if err := cbg.WriteMajorTypeHeader(w, cbg.MajByteString, uint64(len(*c))); err != nil {
		return err
	}
	_, err := w.Write(*c)
	return err
}

func (c *CborByteArray) UnmarshalCBOR(r io.Reader) error {
	maj, extra, err := cbg.CborReadHeader(r)
	if err != nil {
		return err
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}
	if uint64(cap(*c)) < extra {
		*c = make([]byte, extra)
	}
	if _, err := io.ReadFull(r, *c); err != nil {
		return err
	}
	return nil
}

func Cborstr(s string) *CborByteArray {
	v := CborByteArray(s)
	return &v
}

func (c *CborByteArray) Cid() cid.Cid {
	pref := cid.Prefix{
		Codec:    uint64(cid.DagCBOR),
		MhType:   mh.SHA2_256,
		MhLength: 32,
		Version:  1,
	}
	cc, err := pref.Sum(c.RawData())
	if err != nil {
		return cid.Undef
	}
	return cc
}

func (c *CborByteArray) RawData() []byte {
	buf := new(bytes.Buffer)
	c.MarshalCBOR(buf)
	return buf.Bytes()
}
