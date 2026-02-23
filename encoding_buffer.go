package json

import (
	"bytes"
	"sync"

	"github.com/cybergodev/json/internal"
)

// Buffer pools for custom encoder memory optimization
// Note: Pool is now managed in internal package, keep local vars for compatibility
var (
	encoderBufferPool = sync.Pool{
		New: func() any {
			buf := &bytes.Buffer{}
			buf.Grow(2048)
			return buf
		},
	}
)

func getEncoderBuffer() *bytes.Buffer {
	return internal.GetEncoderBuffer()
}

func putEncoderBuffer(buf *bytes.Buffer) {
	internal.PutEncoderBuffer(buf)
}
