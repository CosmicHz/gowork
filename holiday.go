package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const holidayAPI = "https://date.appworlds.cn/next"

// holiday 是一次查询的结果；Known 为 false 时表示数据不可用。
type holiday struct {
	Known bool
	Date  time.Time
	Name  string
	Days  int
	Icon  string
}

type holidayResponse struct {
	Code int `json:"code"`
	Data *struct {
		Date string `json:"date"`
		Days int    `json:"days"`
		Name string `json:"name"`
	} `json:"data"`
}

type holidayCache struct {
	FetchDate string `json:"fetchDate"`
	Date      string `json:"date"`
	Name      string `json:"name"`
	Days      int    `json:"days"`
}

// nextHoliday 返回下一个节假日：优先用磁盘缓存，缓存过期再请求接口。
// 失败时不覆盖缓存，以便下次重试。
func nextHoliday(now time.Time) holiday {
	today := startOfDay(now)

	if h, ok := readHolidayCache(today); ok {
		return h
	}
	h, err := fetchHoliday(now)
	if err != nil {
		return holiday{Name: "数据获取失败", Icon: "⚠️"}
	}
	writeHolidayCache(today, h)
	return h
}

func fetchHoliday(now time.Time) (holiday, error) {
	// 接口语义是「严格晚于该日期」的下一个放假日，所以拿昨天去查：
	// 只有这样才能在今天恰好放假时命中「就是今天」。
	query := now.AddDate(0, 0, -1).Format("2006-01-02")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(holidayAPI + "?date=" + query)
	if err != nil {
		return holiday{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return holiday{}, fmt.Errorf("接口返回 %s", resp.Status)
	}

	var r holidayResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return holiday{}, err
	}
	if r.Code != 200 || r.Data == nil {
		return holiday{}, errors.New("接口没有可用数据")
	}

	date, err := time.ParseInLocation("2006-01-02", r.Data.Date, time.Local)
	if err != nil {
		return holiday{}, err
	}
	return holiday{
		Known: true,
		Date:  date,
		Name:  r.Data.Name,
		Days:  r.Data.Days,
		Icon:  holidayIcon(r.Data.Name),
	}, nil
}

// readHolidayCache 在缓存的节日日期还没过去时复用，否则重新查询。
func readHolidayCache(today time.Time) (holiday, bool) {
	path, err := holidayCachePath()
	if err != nil {
		return holiday{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return holiday{}, false
	}

	var c holidayCache
	if err := json.Unmarshal(raw, &c); err != nil {
		return holiday{}, false
	}
	date, err := time.ParseInLocation("2006-01-02", c.Date, time.Local)
	if err != nil || date.Before(today) {
		return holiday{}, false
	}
	return holiday{
		Known: true,
		Date:  date,
		Name:  c.Name,
		Days:  c.Days,
		Icon:  holidayIcon(c.Name),
	}, true
}

func writeHolidayCache(today time.Time, h holiday) {
	path, err := holidayCachePath()
	if err != nil {
		return
	}
	c := holidayCache{
		FetchDate: today.Format("2006-01-02"),
		Date:      h.Date.Format("2006-01-02"),
		Name:      h.Name,
		Days:      h.Days,
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func holidayCachePath() (string, error) {
	dir, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "holiday-cache.json"), nil
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
