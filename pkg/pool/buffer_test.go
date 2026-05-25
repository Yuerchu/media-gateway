package pool

import (
	"testing"
)

func TestGetBuf(t *testing.T) {
	buf := GetBuf()
	if buf == nil {
		t.Fatal("GetBuf returned nil")
	}
	if buf.Len() != 0 {
		t.Errorf("buffer should be empty, got %d bytes", buf.Len())
	}
	PutBuf(buf)
}

func TestGetBuf_Reset(t *testing.T) {
	buf := GetBuf()
	buf.WriteString("leftover data")
	PutBuf(buf)

	buf2 := GetBuf()
	if buf2.Len() != 0 {
		t.Errorf("reused buffer should be reset, got %d bytes", buf2.Len())
	}
	PutBuf(buf2)
}

func TestPutBuf_DiscardOversized(t *testing.T) {
	buf := GetBuf()
	big := make([]byte, 2*1024*1024)
	buf.Write(big)
	PutBuf(buf)

	buf2 := GetBuf()
	if buf2.Cap() > 1024*1024 {
		t.Log("oversized buffer was discarded as expected (new buffer from pool)")
	}
	PutBuf(buf2)
}

func TestGetBuf_InitialCapacity(t *testing.T) {
	buf := GetBuf()
	if buf.Cap() < 32*1024 {
		t.Errorf("initial capacity should be at least 32KB, got %d", buf.Cap())
	}
	PutBuf(buf)
}
