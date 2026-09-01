package validate

import (
	"fmt"
	"strings"
)

type NameEntry struct {
	Key, Val string
}

type PathSegment struct {
	Names   []NameEntry
	Index   int
	IsIndex bool
	Key     string
	IsKey   bool
}

func NewFieldSegment(pairs ...string) PathSegment {
	n := make([]NameEntry, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		n = append(n, NameEntry{pairs[i], pairs[i+1]})
	}
	return PathSegment{Names: n}
}

// Falls back to the "go" entry when key is missing.
func (s PathSegment) GetName(key string) string {
	var fallback string
	for _, e := range s.Names {
		if e.Key == key {
			return e.Val
		}
		if e.Key == "go" {
			fallback = e.Val
		}
	}
	return fallback
}

func NewIndexSegment(index int) PathSegment {
	return PathSegment{Index: index, IsIndex: true}
}

func NewKeySegment(key any) PathSegment {
	return PathSegment{Key: fmt.Sprint(key), IsKey: true}
}

type Path []PathSegment

func (p Path) Format(f func(i int, seg PathSegment) string) string {
	var sb strings.Builder
	for i, seg := range p {
		sb.WriteString(f(i, seg))
	}
	return sb.String()
}

func (p Path) String() string {
	return p.Format(GoFormatter)
}

func FormatterFor(key string) func(int, PathSegment) string {
	return func(i int, s PathSegment) string {
		if s.IsIndex {
			return fmt.Sprintf("[%d]", s.Index)
		}
		if s.IsKey {
			return fmt.Sprintf("[%s]", s.Key)
		}
		name := s.GetName(key)
		if i == 0 {
			return name
		}
		return "." + name
	}
}

var (
	GoFormatter   = FormatterFor("go")
	JSONFormatter = FormatterFor("json")
	XMLFormatter  = FormatterFor("xml")
	YAMLFormatter = FormatterFor("yaml")
)

const (
	KindStr   = "str"
	KindValue = "value"
	KindSlice = "slice"
	KindObj   = "obj"
	KindTime  = "time"
)

type ValidationError struct {
	Segments Path
	Tag      string
	Value    string
	Kind     string
	Err      error
}

func (e *ValidationError) PrependSegment(segs ...PathSegment) {
	e.Segments = append(Path(segs), e.Segments...)
}

func (e *ValidationError) Error() string {
	val := ""
	if e.Value != "" {
		val = "=" + e.Value
	}
	kind := ""
	if e.Kind != "" {
		kind = " (" + e.Kind + ")"
	}

	if e.Err != nil {
		return fmt.Sprintf("field '%s' failed on '%s%s'%s: %v", e.Segments, e.Tag, val, kind, e.Err)
	}
	return fmt.Sprintf("field '%s' failed on '%s%s'%s", e.Segments, e.Tag, val, kind)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// Called from generated code only; consumers reading errors via errors.As /
// Unwrap() []error don't need to touch this.
func PrependFrom(errs []error, fromIdx int, segs ...PathSegment) {
	for k := fromIdx; k < len(errs); k++ {
		if ve, ok := errs[k].(*ValidationError); ok {
			ve.PrependSegment(segs...)
		}
	}
}

func AppendSubErrors(dst *[]error, err error, segs ...PathSegment) {
	if me, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range me.Unwrap() {
			if ve, ok := e.(*ValidationError); ok {
				ve.PrependSegment(segs...)
			}
			*dst = append(*dst, e)
		}
		return
	}
	if ve, ok := err.(*ValidationError); ok {
		ve.PrependSegment(segs...)
	}
	*dst = append(*dst, err)
}
