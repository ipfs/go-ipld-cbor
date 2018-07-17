package encoding

import (
	"bytes"
	"io"

	cbor "github.com/polydawn/refmt/cbor"
	"github.com/polydawn/refmt/obj/atlas"
)

type proxyReader struct {
	r io.Reader
}

func (r *proxyReader) Read(b []byte) (int, error) {
	return r.r.Read(b)
}

// Unmarshaller is a reusable CBOR unmarshaller.
type Unmarshaller struct {
	unmarshal *cbor.Unmarshaller
	reader    proxyReader
}

// NewUnmarshallerAtlased creates a new reusable unmarshaller.
func NewUnmarshallerAtlased(atl atlas.Atlas) *Unmarshaller {
	m := new(Unmarshaller)
	m.unmarshal = cbor.NewUnmarshallerAtlased(&m.reader, atl)
	return m
}

// Decode reads a CBOR object from the given reader and decodes it into the
// given object.
func (m *Unmarshaller) Decode(r io.Reader, obj interface{}) error {
	m.reader.r = r
	err := m.unmarshal.Unmarshal(obj)
	m.reader.r = nil
	return err
}

// Unmarshal unmarshals the given CBOR byte slice into the given object.
func (m *Unmarshaller) Unmarshal(b []byte, obj interface{}) error {
	return m.Decode(bytes.NewReader(b), obj)
}

// PooledUnmarshaller is a thread-safe pooled CBOR unmarshaller.
type PooledUnmarshaller struct {
	Count         int
	unmarshallers chan *Unmarshaller
}

// SetAtlas set sets the pool's atlas. It is *not* safe to call this
// concurrently.
func (p *PooledUnmarshaller) SetAtlas(atlas atlas.Atlas) {
	p.unmarshallers = make(chan *Unmarshaller, p.Count)
	for len(p.unmarshallers) < cap(p.unmarshallers) {
		p.unmarshallers <- NewUnmarshallerAtlased(atlas)
	}
}

// Decode decodes an object from the passed reader into the given object using
// the pool of unmarshallers.
func (p *PooledUnmarshaller) Decode(r io.Reader, obj interface{}) error {
	u := <-p.unmarshallers
	err := u.Decode(r, obj)
	p.unmarshallers <- u
	return err
}

// Unmarshal unmarshals the passed object using the pool of unmarshallers.
func (p *PooledUnmarshaller) Unmarshal(b []byte, obj interface{}) error {
	u := <-p.unmarshallers
	err := u.Unmarshal(b, obj)
	p.unmarshallers <- u
	return err
}
