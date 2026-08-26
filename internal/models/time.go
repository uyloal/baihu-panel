package models

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/uyloal/baihu-panel/internal/systime"
)

const TimeFormat = "2006-01-02 15:04:05"

// LocalTime 自定义时间类型，JSON 序列化为 "年-月-日 时:分:秒" 格式
type LocalTime time.Time

func (t LocalTime) MarshalJSON() ([]byte, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return []byte("null"), nil
	}
	// 统一输出为东八区时间
	tt = systime.InCST(tt)
	return []byte(fmt.Sprintf(`"%s"`, tt.Format(TimeFormat))), nil
}

func (t *LocalTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	// 去掉引号
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	tt, err := time.ParseInLocation(TimeFormat, s, time.Local)
	if err != nil {
		// 尝试解析 ISO 格式
		tt, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return err
		}
	}
	*t = LocalTime(tt)
	return nil
}

func (t LocalTime) Value() (driver.Value, error) {
	return time.Time(t), nil
}

func (t *LocalTime) Scan(v interface{}) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case time.Time:
		*t = LocalTime(val)
	case string:
		tt, err := time.ParseInLocation(TimeFormat, val, time.Local)
		if err != nil {
			return err
		}
		*t = LocalTime(tt)
	}
	return nil
}

func (t LocalTime) Time() time.Time {
	return time.Time(t)
}

func Now() LocalTime {
	return LocalTime(systime.Now())
}
