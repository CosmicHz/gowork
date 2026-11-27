package holiday

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// isolateCache 把用户缓存目录指到临时目录，避免读写真实缓存。
func isolateCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("LOCALAPPDATA", dir)
}

func writeTestCache(t *testing.T, version int, events []cachedHolidayEvent) {
	t.Helper()
	path, err := cachePath()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(holidayCache{Version: version, Events: events})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// stubAPI 替换默认 Transport，让 fetch 拿到固定的响应。
func stubAPI(t *testing.T, status int, body string) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCachedHolidayMargin(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.Local)
	event := func(days int, name, typ string) cachedHolidayEvent {
		return cachedHolidayEvent{
			Date: now.AddDate(0, 0, days).Format(time.DateOnly),
			Name: name,
			Type: typ,
		}
	}

	cases := []struct {
		desc     string
		events   []cachedHolidayEvent
		margin   time.Duration
		want     bool
		wantName string
	}{
		{"覆盖期充足", []cachedHolidayEvent{event(30, "测试节", "festival")}, prepareMargin, true, "测试节"},
		{"刚好超过剩余期下限", []cachedHolidayEvent{event(8, "测试节", "festival")}, prepareMargin, true, "测试节"},
		{"等于剩余期下限", []cachedHolidayEvent{event(7, "测试节", "festival")}, prepareMargin, false, ""},
		{"快用完", []cachedHolidayEvent{event(3, "测试节", "festival")}, prepareMargin, false, ""},
		{"不要求剩余期时仍可用", []cachedHolidayEvent{event(3, "测试节", "festival")}, 0, true, "测试节"},
		{
			desc:     "覆盖期看最晚事件而不是下一个节日",
			events:   []cachedHolidayEvent{event(1, "近期节", "festival"), event(30, "远期节", "festival")},
			margin:   prepareMargin,
			want:     true,
			wantName: "近期节",
		},
		{
			desc:     "节日也计入覆盖期",
			events:   []cachedHolidayEvent{event(3, "近期放假", "rest"), event(60, "远期节日", "festival")},
			margin:   prepareMargin,
			want:     true,
			wantName: "近期放假",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			isolateCache(t)
			writeTestCache(t, cacheVersion, c.events)
			h, ok := cachedHoliday(now, c.margin)
			if ok != c.want {
				t.Errorf("cachedHoliday = %v, 想要 %v", ok, c.want)
			}
			if ok && h.Name != c.wantName {
				t.Errorf("Name = %q, 想要 %q", h.Name, c.wantName)
			}
		})
	}
}

func TestCachedHolidayRejectsBrokenCache(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.Local)

	t.Run("版本不符", func(t *testing.T) {
		isolateCache(t)
		writeTestCache(t, cacheVersion-1, []cachedHolidayEvent{{
			Date: now.AddDate(0, 0, 30).Format(time.DateOnly), Name: "测试节", Type: "festival",
		}})
		if _, ok := cachedHoliday(now, 0); ok {
			t.Error("版本不符的缓存应当被丢弃")
		}
	})

	t.Run("JSON 损坏", func(t *testing.T) {
		isolateCache(t)
		path, err := cachePath()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := cachedHoliday(now, 0); ok {
			t.Error("损坏的缓存应当被丢弃")
		}
	})

	t.Run("没有未来节日", func(t *testing.T) {
		isolateCache(t)
		writeTestCache(t, cacheVersion, []cachedHolidayEvent{{
			Date: now.AddDate(0, 0, -1).Format(time.DateOnly), Name: "测试节", Type: "festival",
		}})
		if _, ok := cachedHoliday(now, 0); ok {
			t.Error("全部过期的缓存应当被丢弃")
		}
	})
}

func TestPrepareReusesUsableCache(t *testing.T) {
	isolateCache(t)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.Local)
	writeTestCache(t, cacheVersion, []cachedHolidayEvent{{
		Date: now.AddDate(0, 0, 30).Format(time.DateOnly), Name: "国庆节", Type: "rest",
	}})

	// 不应发起请求；一旦请求就直接让用例失败。
	old := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("缓存可用时不应请求接口")
		return nil, io.EOF
	})
	t.Cleanup(func() { http.DefaultTransport = old })

	h, refreshed, err := Prepare(now)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if refreshed {
		t.Error("缓存可用时不应重新抓取")
	}
	if h.Name != "国庆节" {
		t.Errorf("Name = %q, 想要 国庆节", h.Name)
	}
}

func TestPrepareRefreshesExpiringCache(t *testing.T) {
	isolateCache(t)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.Local)
	// 只剩今天，剩余覆盖期不足。
	writeTestCache(t, cacheVersion, []cachedHolidayEvent{{
		Date: now.Format(time.DateOnly), Name: "测试节", Type: "festival",
	}})
	stubAPI(t, http.StatusOK, `{
		"days": [{"date": "2026-09-29"}],
		"nearby": {"next": [
			{"date": "2026-10-01", "events": [{"name": "国庆节", "type": "legal_rest"}]},
			{"date": "2026-10-02", "events": [{"name": "国庆节", "type": "legal_rest"}]}
		]}
	}`)

	h, refreshed, err := Prepare(now)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !refreshed {
		t.Error("缓存快用完时应当重新抓取")
	}
	if h.Name != "国庆节" || h.Days != 2 {
		t.Errorf("Prepare = %q / %d 天, 想要 国庆节 / 2 天", h.Name, h.Days)
	}

	c, err := loadCache()
	if err != nil {
		t.Fatalf("重新抓取后应当写入缓存：%v", err)
	}
	if len(c.Events) == 0 || c.Events[0].Name != "国庆节" {
		t.Errorf("缓存内容 = %+v, 想要包含 国庆节", c.Events)
	}
}

func TestPrepareFetchError(t *testing.T) {
	isolateCache(t)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.Local)
	writeTestCache(t, cacheVersion, []cachedHolidayEvent{{
		Date: now.Format(time.DateOnly), Name: "测试节", Type: "festival",
	}})
	stubAPI(t, http.StatusInternalServerError, `{}`)

	if _, refreshed, err := Prepare(now); err == nil || refreshed {
		t.Errorf("接口失败时应当返回错误且不标记刷新，得到 refreshed=%v, err=%v", refreshed, err)
	}
}
