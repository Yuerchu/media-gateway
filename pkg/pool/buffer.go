package pool

import (
	"bytes"
	"sync"
)

// bufPool is a sync.Pool for reusable byte buffers, reducing GC pressure
// from repeated io.ReadAll / json.Marshal allocations in the hot path.
var bufPool = sync.Pool{
	New: func() any {
		buf := new(bytes.Buffer)
		buf.Grow(32 * 1024) // 32KB initial capacity
		return buf
	},
}

// GetBuf returns a reset buffer from the pool.
func GetBuf() *bytes.Buffer {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutBuf returns a buffer to the pool. Oversized buffers (>1MB) are discarded.
func PutBuf(buf *bytes.Buffer) {
	if buf.Cap() > 1024*1024 {
		return
	}
	bufPool.Put(buf)
}
