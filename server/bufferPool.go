package server

import (
	"bytes"
	"sync"
)

// BufferPool provides an API for managing
// bytes.Buffer objects:
type BufferPool interface {
	GetBuffer() *bytes.Buffer
	PutBuffer(*bytes.Buffer)
}

// syncPoolBufPool is an implementation of BufferPool
// that uses a sync.Pool to maintain buffers:
type syncPoolBufPool struct {
	pool       *sync.Pool
	makeBuffer func() interface{}
}

func NewSyncPool(buf_size int) BufferPool {
	var newPool syncPoolBufPool

	newPool.makeBuffer = func() interface{} {
		var b bytes.Buffer
		b.Grow(buf_size)
		return &b
	}
	newPool.pool = &sync.Pool{}
	newPool.pool.New = newPool.makeBuffer

	return &newPool
}

func (bp *syncPoolBufPool) GetBuffer() (b *bytes.Buffer) {
	pool_object := bp.pool.Get()

	b, ok := pool_object.(*bytes.Buffer)
	if !ok { // explicitly make buffer if sync.Pool returns nil:
		b = bp.makeBuffer().(*bytes.Buffer)
	}
	return
}

func (bp *syncPoolBufPool) PutBuffer(b *bytes.Buffer) {
 	b.Reset()
	bp.pool.Put(b)
}