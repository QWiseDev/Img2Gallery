package timeutil

import "time"

const format = "2006-01-02 15:04:05"

func LocalTimestamp(locationName string) string {
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		loc, _ = time.LoadLocation("Asia/Shanghai")
	}
	return time.Now().In(loc).Format(format)
}
