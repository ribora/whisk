package validate

import (
	"fmt"
	"strconv"
)

func IsOneof(value any, args ...string) bool {
	var v string
	switch val := value.(type) {
	case string:
		v = val
	case []byte:
		v = string(val)
	case bool:
		v = strconv.FormatBool(val)
	case int:
		v = strconv.Itoa(val)
	case int8:
		v = strconv.FormatInt(int64(val), 10)
	case int16:
		v = strconv.FormatInt(int64(val), 10)
	case int32:
		v = strconv.FormatInt(int64(val), 10)
	case int64:
		v = strconv.FormatInt(val, 10)
	case uint:
		v = strconv.FormatUint(uint64(val), 10)
	case uint8:
		v = strconv.FormatUint(uint64(val), 10)
	case uint16:
		v = strconv.FormatUint(uint64(val), 10)
	case uint32:
		v = strconv.FormatUint(uint64(val), 10)
	case uint64:
		v = strconv.FormatUint(val, 10)
	case float32:
		v = strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		v = strconv.FormatFloat(val, 'f', -1, 64)
	case fmt.Stringer:
		v = val.String()
	default:
		v = fmt.Sprint(value)
	}

	for _, arg := range args {
		if v == arg {
			return true
		}
	}
	return false
}
