package core_test

import (
	"sync"
	"sync/atomic"
	"testing"
)

const maxValue = 0xfffff

type MutexCounter struct {
	sync.Mutex
	value uint32
}

func (c *MutexCounter) Next() uint32 {
	c.Lock()
	defer c.Unlock()
	c.value++
	if c.value > 0xfffff {
		c.value = 1
	}
	return c.value
}

type AtomicCounter struct {
	value atomic.Uint32
}

func (c *AtomicCounter) Next() uint32 {
again:
	old := c.value.Load()
	new := old + 1
	if new > maxValue {
		new = 1
	}
	if c.value.CompareAndSwap(old, new) {
		return new
	}
	goto again
}

const parallelism = 64

func BenchmarkMutex(b *testing.B) {
	var counter MutexCounter
	b.SetParallelism(parallelism)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Next()
		}
	})
}

func BenchmarkAtomic(b *testing.B) {
	var counter AtomicCounter
	b.SetParallelism(parallelism)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Next()
		}
	})
}
