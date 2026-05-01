package timeutil

import "time"

const format = "2006-01-02 15:04:05"

func LocalTimestamp(locationName string) string {
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		loc, err = time.LoadLocation("Asia/Shanghai")
	}
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return time.Now().In(loc).Format(format)
}
