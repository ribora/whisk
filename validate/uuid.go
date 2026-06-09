package validate

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/google/uuid"
)

func IsUuid[Uuid ~string | uuid.UUID | *uuid.UUID](u Uuid, args ...string) (_ bool, err error) {
	var id uuid.UUID
	switch e := any(u).(type) {
	case uuid.UUID:
		id = e
	case *uuid.UUID:
		id = *e
	case string:
		id, err = uuid.Parse(e)
	default:
		id, err = uuid.Parse(reflect.ValueOf(e).String())
	}
	if err != nil {
		return false, err
	}
	if len(args) > 0 {
		version, err := strconv.Atoi(args[0])
		if err != nil {
			return false, err
		}
		if id.Version() != uuid.Version(version) {
			return false, fmt.Errorf("uuid version not match: expected %d but %d given", version, id.Version())
		}
	}
	return true, nil
}
