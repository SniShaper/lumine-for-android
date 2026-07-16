package core

import "sync/atomic"

type Counter struct {
	v atomic.Uint32
}

func (c *Counter) Next() uint32 {
again:
	old := c.v.Load()
	new := old + 1
	if new > maxConnID {
		new = 1
	}
	if c.v.CompareAndSwap(old, new) {
		return new
	}
	goto again
}
