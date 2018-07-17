package encoding

import (
	"sync"

	refmt "github.com/polydawn/refmt"
	"github.com/polydawn/refmt/obj/atlas"
)

// PooledCloner is a thread-safe pooled object cloner.
type PooledCloner struct {
	pool sync.Pool
}

// SetAtlas set sets the pool's atlas. It is *not* safe to call this
// concurrently.
func (p *PooledCloner) SetAtlas(atl atlas.Atlas) {
	p.pool = sync.Pool{
		New: func() interface{} {
			return refmt.NewCloner(atl)
		},
	}
}

// Clone clones a into b using a cloner from the pool.
func (p *PooledCloner) Clone(a, b interface{}) error {
	c := p.pool.Get().(refmt.Cloner)
	err := c.Clone(a, b)
	p.pool.Put(c)
	return err
}
