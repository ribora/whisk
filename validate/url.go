package validate

import (
	"net/url"
	"reflect"
)

func IsUrl[Url url.URL | *url.URL | ~string](u Url, args ...string) bool {
	var v string
	switch e := any(u).(type) {
	case url.URL:
		return true
	case *url.URL:
		return true
	case string:
		v = e
	default:
		v = reflect.ValueOf(e).String()
	}
	_, err := url.ParseRequestURI(v)
	return err == nil
}
