package bufferpool

import "sync"

type BufferPool struct {
	pool sync.Pool
}

func New(baseCapacity int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() any {
				buf := make([]byte, 0, baseCapacity)
				return &buf
			},
		},
	}
}

func (bp *BufferPool) Get() []byte {
	return *bp.pool.Get().(*[]byte)
}

func (bp *BufferPool) Put(buf []byte) {
	bp.pool.Put(&buf)
}
