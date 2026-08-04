package pkg

import "time"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/12 16:52:04
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/12 16:52:04
 */

func NowFormatSecond() string {
	return TimeFormatSecond(time.Now())
}

func NowFormatMinute() string {
	return TimeFormatMinute(time.Now())
}

func NowFormatHour() string {
	return TimeFormatHour(time.Now())
}

func NowFormatDay() string {
	return TimeFormatDay(time.Now())
}

func NowFormatMonth() string {
	return TimeFormatMonth(time.Now())
}

func NowFormatYear() string {
	return TimeFormatYear(time.Now())
}

func TimeFormatSecond(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func TimeFormatMinute(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

func TimeFormatHour(t time.Time) string {
	return t.Format("2006-01-02 15")
}

func TimeFormatDay(t time.Time) string {
	return t.Format("2006-01-02")
}

func TimeFormatMonth(t time.Time) string {
	return t.Format("2006-01")
}

func TimeFormatYear(t time.Time) string {
	return t.Format("2006")
}

func NowStartDay() time.Time {
	return TimeStartDay(time.Now())
}

func NowEndDay() time.Time {
	return TimeEndDay(time.Now())
}

func TimeStartDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func TimeEndDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}
