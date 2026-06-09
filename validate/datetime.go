package validate

import (
	"strconv"
	"strings"
	"time"
)

func IsDatetime(value string, args ...string) bool {
	format := time.RFC3339
	if len(args) > 0 {
		format = args[0]
	}
	_, err := time.Parse(format, value)
	return err == nil
}

func IsTimeMin(value any, targetStr string) bool {
	t, target, hasTime, ok := prepareCompare(value, targetStr)
	if !ok {
		return false
	}
	if !hasTime {
		startOfDay := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, target.Location())
		return !t.Before(startOfDay)
	}
	return !t.Before(target)
}

func IsTimeMax(value any, targetStr string) bool {
	t, target, hasTime, ok := prepareCompare(value, targetStr)
	if !ok {
		return false
	}
	if !hasTime {
		endOfDay := time.Date(target.Year(), target.Month(), target.Day(), 23, 59, 59, int(time.Second-1), target.Location())
		return !t.After(endOfDay)
	}
	return !t.After(target)
}

func IsTimeGT(value any, targetStr string) bool {
	t, target, _, ok := prepareCompare(value, targetStr)
	if !ok {
		return false
	}
	return t.After(target)
}

func IsTimeLT(value any, targetStr string) bool {
	t, target, _, ok := prepareCompare(value, targetStr)
	if !ok {
		return false
	}
	return t.Before(target)
}

func prepareCompare(value any, targetStr string) (time.Time, time.Time, bool, bool) {
	var t time.Time
	var valOk, hasTime bool
	switch v := value.(type) {
	case time.Time:
		t, valOk = v, true
	case *time.Time:
		if v != nil {
			t, valOk = *v, true
		}
	case string:
		if tv, err := time.Parse(time.RFC3339, v); err == nil {
			t, valOk, hasTime = tv, true, true
		} else if tv, err := time.Parse("2006-01-02", v); err == nil {
			t, valOk = tv, true
		}
	}
	if !valOk {
		return t, t, false, false
	}

	if targetStr == "now" {
		return t, time.Now(), hasTime, true
	}

	if len(targetStr) >= 2 {
		unit := ""
		valStr := ""
		if strings.HasSuffix(targetStr, "days") {
			unit, valStr = "d", strings.TrimSuffix(targetStr, "days")
		} else if strings.HasSuffix(targetStr, "d") {
			unit, valStr = "d", strings.TrimSuffix(targetStr, "d")
		} else if strings.HasSuffix(targetStr, "h") {
			unit, valStr = "h", strings.TrimSuffix(targetStr, "h")
		} else if strings.HasSuffix(targetStr, "m") {
			unit, valStr = "m", strings.TrimSuffix(targetStr, "m")
		}

		if val, err := strconv.Atoi(valStr); err == nil {
			now := time.Now()
			switch unit {
			case "d":
				return t, now.AddDate(0, 0, val), false, true
			case "h":
				return t, now.Add(time.Duration(val) * time.Hour), true, true
			case "m":
				return t, now.Add(time.Duration(val) * time.Minute), true, true
			}
		}
	}

	if target, err := time.Parse(time.RFC3339, targetStr); err == nil {
		return t, target, true, true
	}
	if target, err := time.Parse("2006-01-02", targetStr); err == nil {
		return t, target, false, true
	}

	return t, t, false, false
}
