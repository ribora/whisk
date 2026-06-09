package validate

import "encoding/json"

func IsJson(value string, args ...string) bool {
	return json.Valid([]byte(value))
}
