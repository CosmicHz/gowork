package main

import (
	"testing"
	"time"
)

// holidayFixture 构造一份日视图响应：today 为当天明细，next 为之后的节日分组。
// previous 由需要它的用例自行补上。
func holidayFixture(today holidayDay, next ...holidayGroup) holidayResponse {
	var r holidayResponse
	if today.Date != "" {
		r.Days = []holidayDay{today}
	}
	r.Nearby.Next = next
	return r
}

func restGroup(date, name string) holidayGroup {
	return eventGroup(date, name, "legal_rest")
}

func eventGroup(date, name, typ string) holidayGroup {
	return holidayGroup{Date: date, Events: []holidayEvent{{Name: name, Type: typ}}}
}

func TestParseHoliday(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.Local)

	cases := []struct {
		desc      string
		now       time.Time
		resp      holidayResponse
		wantName  string
		wantDate  string
		wantDays  int
		wantError bool
	}{
		{
			desc: "取最近的下一个放假日并统计连续天数",
			resp: holidayFixture(
				holidayDay{Date: "2026-09-29"},
				restGroup("2026-10-01", "国庆节"),
				restGroup("2026-10-02", "国庆节"),
				restGroup("2026-10-03", "国庆节"),
			),
			wantName: "国庆节",
			wantDate: "2026-10-01",
			wantDays: 3,
		},
		{
			desc: "今天就是放假日",
			resp: holidayFixture(
				holidayDay{Date: "2026-10-01", LegalHolidayName: "国庆节", LegalHolidayType: "rest"},
				restGroup("2026-10-02", "国庆节"),
			),
			wantName: "国庆节",
			wantDate: "2026-10-01",
			wantDays: 2,
		},
		{
			desc: "假期中途报的是整段天数",
			now:  time.Date(2026, 10, 2, 10, 0, 0, 0, time.Local),
			resp: func() holidayResponse {
				r := holidayFixture(
					holidayDay{Date: "2026-10-02", LegalHolidayName: "国庆节", LegalHolidayType: "rest"},
					restGroup("2026-10-03", "国庆节"),
				)
				r.Nearby.Previous = []holidayGroup{
					restGroup("2026-09-30", "国庆节"),
					restGroup("2026-10-01", "国庆节"),
				}
				return r
			}(),
			wantName: "国庆节",
			wantDate: "2026-10-02",
			wantDays: 4,
		},
		{
			desc: "调休上班不算放假",
			resp: holidayFixture(
				holidayDay{Date: "2026-09-29"},
				holidayGroup{Date: "2026-10-10", Events: []holidayEvent{{Name: "国庆节", Type: "legal_workday_adjust"}}},
				restGroup("2026-11-01", "元旦"),
			),
			wantName: "元旦",
			wantDate: "2026-11-01",
			wantDays: 1,
		},
		{
			desc: "已经过去的节日被忽略",
			resp: holidayFixture(
				holidayDay{Date: "2026-09-29"},
				restGroup("2026-09-26", "中秋节"),
				restGroup("2026-10-01", "国庆节"),
			),
			wantName: "国庆节",
			wantDate: "2026-10-01",
			wantDays: 1,
		},
		{
			desc: "无法定排期时退回最近的公历节日",
			resp: holidayFixture(
				holidayDay{Date: "2026-10-24"},
				eventGroup("2026-11-07", "立冬", "solar_term"),
				eventGroup("2027-01-01", "元旦", "solar_festival"),
			),
			wantName: "元旦",
			wantDate: "2027-01-01",
			wantDays: 0,
		},
		{
			desc: "农历节日也算，节气不算",
			resp: holidayFixture(
				holidayDay{Date: "2026-10-24"},
				eventGroup("2026-11-07", "立冬", "solar_term"),
				eventGroup("2026-12-22", "冬至节", "lunar_festival"),
			),
			wantName: "冬至节",
			wantDate: "2026-12-22",
			wantDays: 0,
		},
		{
			desc: "法定放假日优先于更早的节日",
			resp: holidayFixture(
				holidayDay{Date: "2026-09-29"},
				eventGroup("2026-09-30", "重阳节", "lunar_festival"),
				restGroup("2026-10-01", "国庆节"),
			),
			wantName: "国庆节",
			wantDate: "2026-10-01",
			wantDays: 1,
		},
		{
			desc:      "没有可用的放假日",
			resp:      holidayFixture(holidayDay{Date: "2026-09-29"}),
			wantError: true,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			at := c.now
			if at.IsZero() {
				at = now
			}
			h, err := parseHoliday(c.resp, at)
			if c.wantError {
				if err == nil {
					t.Fatal("想要错误，得到 nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("意外错误：%v", err)
			}
			if h.Name != c.wantName {
				t.Errorf("Name = %q, 想要 %q", h.Name, c.wantName)
			}
			if got := h.Date.Format("2006-01-02"); got != c.wantDate {
				t.Errorf("Date = %q, 想要 %q", got, c.wantDate)
			}
			if h.Days != c.wantDays {
				t.Errorf("Days = %d, 想要 %d", h.Days, c.wantDays)
			}
		})
	}
}
