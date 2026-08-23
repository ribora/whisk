package validate

import (
	"fmt"
	"reflect"
	"strconv"
	"uuid"

	guid "github.com/google/uuid"
)

func IsUuid[Uuid ~string | uuid.UUID | *uuid.UUID | guid.UUID | *guid.UUID](u Uuid, args ...string) (_ bool, err error) {
	var id guid.UUID
	switch e := any(u).(type) {
	case guid.UUID:
		id = e
	case *guid.UUID:
		id = *e
	case uuid.UUID:
		id = guid.UUID(e)
	case *uuid.UUID:
		id = guid.UUID(*e)
	case string:
		id, err = guid.Parse(e)
	default:
		id, err = guid.Parse(reflect.ValueOf(e).String())
	}
	if err != nil {
		return false, err
	}
	if len(args) > 0 {
		version, err := strconv.Atoi(args[0])
		if err != nil {
			return false, err
		}
		if id.Version() != guid.Version(version) {
			return false, fmt.Errorf("uuid version not match: expected %d but %d given", version, id.Version())
		}
	}
	return true, nil
}
