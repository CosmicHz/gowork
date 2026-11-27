package render

import (
	"net/http"
	"os"
	"testing"
)

// blockTransport 让测试里的真实网络请求立即暴露，而不是悄悄发出。
type blockTransport struct{}

func (blockTransport) RoundTrip(*http.Request) (*http.Response, error) {
	panic("测试中禁止真实网络请求：仪表盘测试请关闭 showHoliday，或替换 http.DefaultTransport")
}

// TestMain 默认封锁网络，防止 Render 里的 holiday.Next 真去请求接口。
func TestMain(m *testing.M) {
	old := http.DefaultTransport
	http.DefaultTransport = blockTransport{}
	code := m.Run()
	http.DefaultTransport = old
	os.Exit(code)
}

// TestNetworkBlocked 确认默认 transport 真的会拦住请求，避免封锁被误删。
func TestNetworkBlocked(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("测试默认 transport 应当封锁网络")
		}
	}()
	_, _ = http.Get("https://example.com")
}
