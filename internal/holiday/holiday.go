// Package holiday 负责节假日查询、磁盘缓存与仪表盘文案。
package holiday

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gowork/internal/config"
)

const holidayAPI = "https://uapis.cn/api/v1/misc/holiday-calendar"

// cacheVersion 变更数据源或缓存结构时递增；版本不匹配的缓存直接丢弃。
const cacheVersion = 4

// Holiday 是一次查询的结果；Known 为 false 时表示数据不可用。
type Holiday struct {
	Known bool
	Date  time.Time
	Name  string
	Days  int
	Icon  string
}

type holidayResponse struct {
	Days   []holidayDay  `json:"days"`
	Nearby holidayNearby `json:"nearby"`
}

type holidayNearby struct {
	Previous []holidayGroup `json:"previous"`
	Next     []holidayGroup `json:"next"`
}

type holidayDay struct {
	Date             string `json:"date"`
	LegalHolidayName string `json:"legal_holiday_name"`
	LegalHolidayType string `json:"legal_holiday_type"`
	SolarFestival    string `json:"solar_festival"`
	LunarFestival    string `json:"lunar_festival"`
}

type holidayGroup struct {
	Date   string         `json:"date"`
	Events []holidayEvent `json:"events"`
}

type holidayEvent struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// 整理过的节日记录。
// 每条记录是「日期 + 名称 + 类型」，后续运行据此重新挑选最近的节日。
type holidayCache struct {
	Version int                  `json:"version"`
	Events  []cachedHolidayEvent `json:"events"`
}

type cachedHolidayEvent struct {
	Date string `json:"date"`
	Name string `json:"name"`
	Type string `json:"type"` // rest（法定放假）或 festival（公历/农历节日）
}

// Next 返回下一个放假日或节日：先从缓存里的节日记录中挑，全部过期后才请求接口。
// 查询失败不写缓存，下次运行仍会重试。
func Next(now time.Time) Holiday {
	if h, ok := readCache(now); ok {
		return h
	}
	resp, err := fetch(now)
	if err != nil {
		return Holiday{Name: "数据获取失败", Icon: "⚠️"}
	}
	h, err := parseHoliday(resp, now)
	if err != nil {
		return Holiday{Name: "数据获取失败", Icon: "⚠️"}
	}
	writeCache(resp)
	return h
}

// Line 把查询结果渲染成仪表盘「节日」行的一句话。
func Line(h Holiday, now time.Time) string {
	if !h.Known {
		return h.Name + h.Icon
	}

	days := int(h.Date.Sub(startOfDay(now)) / (24 * time.Hour))
	var msg string
	switch {
	case days == 0:
		msg = "就是今天！快乐！"
	case days == 1:
		msg = "就在明天！"
	case days <= 3:
		msg = fmt.Sprintf("还有 %d 天，倒计时开始！", days)
	default:
		msg = fmt.Sprintf("还有 %d 天", days)
	}
	if h.Days > 0 {
		msg += fmt.Sprintf("  (放假 %d 天)", h.Days)
	}
	return h.Name + h.Icon + "  " + msg
}

// fetch 按天查询今天的万年历和节日，一次请求拿回当天与前后两段节日。
// 取到的节日包含公历、农历节日和节气。
// API 接受一个 exclude_past 参数，文档写的是“过滤今天之前已经过去的节日”，
// 但是我们不传，因为它所谓的“今天”是服务器时间，不是我们传的 date，传的话可能导致困惑，
// 特此记录。
func fetch(now time.Time) (holidayResponse, error) {
	q := url.Values{}
	q.Set("date", now.Format("2006-01-02"))
	q.Set("holiday_type", "all")
	q.Set("include_nearby", "true")
	q.Set("nearby_limit", "30")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(holidayAPI + "?" + q.Encode())
	if err != nil {
		return holidayResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return holidayResponse{}, fmt.Errorf("接口返回 %s", resp.Status)
	}

	var r holidayResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return holidayResponse{}, err
	}
	return r, nil
}

// parseHoliday 从按天查询的结果里挑出今天或之后的节日。
// 优先取法定节假日（legal_rest）并统计整段放假天数；法定排期还没公布时，
// 退回最近的一个公历/农历节日，只是没有放假天数可报。节气不算节日。
func parseHoliday(r holidayResponse, now time.Time) (Holiday, error) {
	rest, festivals := eventMaps(collectEvents(r))
	return selectHoliday(rest, festivals, now)
}

// collectEvents 把取到的“节日列表”与“前后最近节日”（放假天数要向前补齐）整理成去重后的记录列表。
func collectEvents(r holidayResponse) []cachedHolidayEvent {
	seen := make(map[string]bool)
	var events []cachedHolidayEvent
	add := func(date, name, typ string) {
		if date == "" || name == "" || seen[date+"/"+typ] {
			return
		}
		seen[date+"/"+typ] = true
		events = append(events, cachedHolidayEvent{Date: date, Name: name, Type: typ})
	}

	if len(r.Days) > 0 {
		d := r.Days[0]
		if d.LegalHolidayType == "rest" {
			add(d.Date, d.LegalHolidayName, "rest")
		}
		if name := firstNames(d.SolarFestival, d.LunarFestival); name != "" {
			add(d.Date, name, "festival")
		}
	}
	for _, groups := range [][]holidayGroup{r.Nearby.Previous, r.Nearby.Next} {
		for _, g := range groups {
			for _, e := range g.Events {
				switch e.Type {
				case "legal_rest":
					add(g.Date, e.Name, "rest")
				case "solar_festival", "lunar_festival":
					add(g.Date, e.Name, "festival")
				}
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].Date != events[j].Date {
			return events[i].Date < events[j].Date
		}
		return events[i].Type < events[j].Type
	})
	return events
}

// eventMaps 把记录列表拆成「日期 -> 名称」的两张表。
func eventMaps(events []cachedHolidayEvent) (rest, festivals map[string]string) {
	rest = make(map[string]string)
	festivals = make(map[string]string)
	for _, e := range events {
		switch e.Type {
		case "rest":
			rest[e.Date] = e.Name
		case "festival":
			festivals[e.Date] = e.Name
		}
	}
	return rest, festivals
}

// selectHoliday 在两张日期表里挑出今天或之后最近的一个，并算出放假天数。
func selectHoliday(rest, festivals map[string]string, now time.Time) (Holiday, error) {
	today := startOfDay(now)

	if start, name := earliestFrom(rest, today); name != "" {
		return Holiday{
			Known: true,
			Date:  start,
			Name:  name,
			Days:  restRun(rest, start),
			Icon:  holidayIcon(name),
		}, nil
	}
	if start, name := earliestFrom(festivals, today); name != "" {
		return Holiday{
			Known: true,
			Date:  start,
			Name:  name,
			Icon:  holidayIcon(name),
		}, nil
	}
	return Holiday{}, errors.New("接口没有可用的节日")
}

// firstNames 返回第一个非空名字。
func firstNames(names ...string) string {
	for _, name := range names {
		if name != "" {
			return name
		}
	}
	return ""
}

// earliestFrom 返回不早于 today 的最近日期及其名称。
func earliestFrom(dates map[string]string, today time.Time) (time.Time, string) {
	var start time.Time
	name := ""
	for date, n := range dates {
		d, err := time.ParseInLocation("2006-01-02", date, time.Local)
		if err != nil || d.Before(today) {
			continue
		}
		if start.IsZero() || d.Before(start) {
			start, name = d, n
		}
	}
	return start, name
}

// restRun 返回 start 所在的那段连续放假一共多少天：先向前找到头，再往后数。
// 用整段长度而不是剩余天数，这样假期中途看到的也是「放假 N 天」。
func restRun(rest map[string]string, start time.Time) int {
	first := start
	for {
		prev := first.AddDate(0, 0, -1).Format("2006-01-02")
		if _, ok := rest[prev]; !ok {
			break
		}
		first = first.AddDate(0, 0, -1)
	}

	n := 0
	for d := first; ; d = d.AddDate(0, 0, 1) {
		if _, ok := rest[d.Format("2006-01-02")]; !ok {
			return n
		}
		n++
	}
}

// readCache 用缓存里整理好的记录重新挑选。缓存里还有未来的节日就复用，
// 全部过去之后（挑不出结果）才返回 false，让调用方重新请求。
func readCache(now time.Time) (Holiday, bool) {
	path, err := cachePath()
	if err != nil {
		return Holiday{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Holiday{}, false
	}

	var c holidayCache
	if err := json.Unmarshal(raw, &c); err != nil {
		return Holiday{}, false
	}
	if c.Version != cacheVersion {
		return Holiday{}, false
	}

	rest, festivals := eventMaps(c.Events)
	h, err := selectHoliday(rest, festivals, now)
	if err != nil {
		return Holiday{}, false
	}
	return h, true
}

// writeCache 把接口返回整理成记录列表后落盘，不保留原始响应。
func writeCache(r holidayResponse) {
	path, err := cachePath()
	if err != nil {
		return
	}
	c := holidayCache{
		Version: cacheVersion,
		Events:  collectEvents(r),
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func cachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "holiday-cache.json"), nil
}

// startOfDay 把时间截到当天 0 点，日期差按自然日计算。
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func holidayIcon(name string) string {
	switch {
	case strings.Contains(name, "元旦"):
		return "🎉"
	case strings.Contains(name, "春节"):
		return "🧧"
	case strings.Contains(name, "清明"):
		return "🌿"
	case strings.Contains(name, "劳动"):
		return "🔧"
	case strings.Contains(name, "端午"):
		return "🐉"
	case strings.Contains(name, "中秋"):
		return "🥮"
	case strings.Contains(name, "国庆"):
		return "🇨🇳"
	case strings.Contains(name, "情人"):
		return "💝"
	case strings.Contains(name, "妇女"):
		return "🌸"
	case strings.Contains(name, "愚人"):
		return "🤡"
	case strings.Contains(name, "儿童"):
		return "🍭"
	case strings.Contains(name, "圣诞"):
		return "🎄"
	case strings.Contains(name, "万圣"):
		return "🎃"
	default:
		return "📅"
	}
}
