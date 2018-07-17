package encoding

import (
	refmt "github.com/polydawn/refmt"
	"github.com/polydawn/refmt/obj/atlas"
)

// PooledCloner is a thread-safe pooled object cloner.
type PooledCloner struct {
	Count   int
	cloners chan refmt.Cloner
}

// SetAtlas set sets the pool's atlas. It is *not* safe to call this
// concurrently.
func (p *PooledCloner) SetAtlas(atlas atlas.Atlas) {
	p.cloners = make(chan refmt.Cloner, p.Count)
	for len(p.cloners) < cap(p.cloners) {
		p.cloners <- refmt.NewCloner(atlas)
	}
}

// Clone clones a into b using a cloner from the pool.
func (p *PooledCloner) Clone(a, b interface{}) error {
	c := <-p.cloners
	err := c.Clone(a, b)
	p.cloners <- c
	return err
}
