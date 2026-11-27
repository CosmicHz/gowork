package testnet

import (
	"net/http"
	"testing"
)

// TestBlockingTransportPanics 确认封锁真的生效，避免辅助代码被改坏后静默放行。
func TestBlockingTransportPanics(t *testing.T) {
	old := http.DefaultTransport
	http.DefaultTransport = blockingTransport{}
	t.Cleanup(func() { http.DefaultTransport = old })

	defer func() {
		if recover() == nil {
			t.Error("默认 transport 应当封锁网络请求")
		}
	}()
	_, _ = http.Get("https://example.com")
}
