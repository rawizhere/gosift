package repo

import "time"

func timeParse(v string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", v)
	if err != nil {
		return time.Time{}
	}
	return t
}
