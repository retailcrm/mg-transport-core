package logger

import (
	"encoding/binary"
	"sync/atomic"
	"time"
	"unsafe"
)

var streamIDCounter = uint64(time.Now().Unix())

func hexEncode(dst *[32]byte, src *[8]byte, cnt uint64) int {
	const hexDigits = "0123456789abcdef"
	for i, b := range src[:] {
		dst[i*2] = hexDigits[b>>4]
		dst[i*2+1] = hexDigits[b&0x0F]
	}
	idx := 16
	for cnt > 0 {
		dst[idx] = hexDigits[cnt&0xF]
		cnt >>= 4
		idx++
	}
	for i, j := 16, idx-1; i < j; i, j = i+1, j-1 {
		dst[i], dst[j] = dst[j], dst[i]
	}
	return idx
}

func bytesToString(b *[32]byte, length int) string {
	return unsafe.String(&b[0], length)
}

func generateStreamID() string {
	var id [8]byte
	var hexID [32]byte

	cnt := atomic.AddUint64(&streamIDCounter, 1)
	timestamp := time.Now().UnixNano()
	binary.BigEndian.PutUint64(id[:], uint64(timestamp))
	length := hexEncode(&hexID, &id, cnt)

	return bytesToString(&hexID, length)
}
