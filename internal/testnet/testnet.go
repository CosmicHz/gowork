// Package testnet 在测试中封锁真实网络请求，避免用例悄悄依赖外网。
package testnet

import (
	"fmt"
	"net/http"
	"os"
	"testing"
)

// realTransport 保存封锁前的默认 Transport，供 AllowReal 恢复。
var realTransport http.RoundTripper

// Block 在封锁网络的前提下运行测试并返回退出码；在 TestMain 里这样用：
//
//	func TestMain(m *testing.M) { os.Exit(testnet.Block(m)) }
//
// 任何走默认 Transport 的请求都会 panic，让对应用例失败，而不是真的联网。
func Block(m *testing.M) int {
	realTransport = http.DefaultTransport
	http.DefaultTransport = blockingTransport{}
	code := m.Run()
	http.DefaultTransport = realTransport
	return code
}

// AllowReal 允许当前测试访问真实网络，供明确的集成测试使用。
// 默认直接 skip；设置 GOWORK_TEST_NET=1 才真正放行，避免 CI 意外依赖外网。
// 它改的是进程级默认 Transport，不能与 t.Parallel 同时使用。
//
// 只是需要一个假的 HTTP 响应时不要用它：自定义 Transport，或用
// httptest.Server 的 srv.Client() 都可以。
func AllowReal(t *testing.T) {
	t.Helper()
	if os.Getenv("GOWORK_TEST_NET") == "" {
		t.Skip("需要真实网络：设置 GOWORK_TEST_NET=1 后重跑")
	}
	if realTransport == nil {
		t.Fatal("testnet.Block 没有在 TestMain 里调用")
	}
	http.DefaultTransport = realTransport
	t.Cleanup(func() { http.DefaultTransport = blockingTransport{} })
}

type blockingTransport struct{}

func (blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	panic(fmt.Sprintf("测试中禁止真实网络请求：%s %s；需要响应时请替换 http.DefaultTransport", req.Method, req.URL))
}
