package validate

import (
	"errors"
	"net"
	"net/mail"
	"reflect"
	"strings"
)

func IsEmail[Email mail.Address | *mail.Address | ~string](email Email, args ...string) (_ bool, err error) {
	var addr *mail.Address
	switch e := any(email).(type) {
	case mail.Address:
		addr = &e
	case *mail.Address:
		addr = e
	case string:
		addr, err = mail.ParseAddress(e)
	default:
		addr, err = mail.ParseAddress(reflect.ValueOf(e).String())
	}
	if err != nil {
		return false, err
	}
	for _, a := range args {
		switch a {
		case "dns":
			at := strings.LastIndex(addr.Address, "@")
			domain := addr.Address[at+1:]
			mx, err := net.LookupMX(domain)
			if err != nil {
				return false, err
			}
			return len(mx) > 0, nil
		default:
			return false, errors.New("unknown email argument: " + a)
		}
	}
	return true, nil
}
