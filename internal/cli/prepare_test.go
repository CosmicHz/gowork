package cli

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gowork/internal/config"
	"gowork/internal/console"
)

type stubTransport func(*http.Request) (*http.Response, error)

func (f stubTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRunSubcommandPrepareRejectsArgs(t *testing.T) {
	con := console.NewWith(io.Discard, io.Discard, false, false)
	handled, err := runSubcommand([]string{"gowork", "prepare", "extra"}, con)
	if !handled {
		t.Fatal("prepare 应当被当作子命令")
	}
	if err == nil || !strings.Contains(err.Error(), "不接受参数") {
		t.Errorf("错误 = %v, 想要包含「不接受参数」", err)
	}
}

func TestPrepareSubcommandReusesCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("LOCALAPPDATA", dir)

	cacheDir, err := config.CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	date := time.Now().AddDate(0, 0, 30).Format(time.DateOnly)
	payload := fmt.Sprintf(`{"version":4,"events":[{"date":%q,"name":"测试节","type":"festival"}]}`, date)
	if err := os.WriteFile(filepath.Join(cacheDir, "holiday-cache.json"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	con := console.NewWith(&out, io.Discard, false, false)

	// 缓存充足时不应联网；一旦请求就报错并返回失败，保证用例快速、确定地挂掉。
	old := http.DefaultTransport
	http.DefaultTransport = stubTransport(func(*http.Request) (*http.Response, error) {
		t.Error("缓存可用时不应请求接口")
		return nil, io.EOF
	})
	t.Cleanup(func() { http.DefaultTransport = old })

	handled, err := runSubcommand([]string{"gowork", "prepare"}, con)
	if !handled {
		t.Fatal("prepare 应当被当作子命令")
	}
	if err != nil {
		t.Fatalf("runSubcommand: %v", err)
	}
	if !strings.Contains(out.String(), "仍然可用") {
		t.Errorf("输出 = %q, 想要包含「仍然可用」", out.String())
	}
}
