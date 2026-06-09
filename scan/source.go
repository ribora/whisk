package scan

import "context"

// Source resolves values from named scopes. Scope is an opaque string
// — HTTP defines "path"/"query"/"header"/"cookie"/"form"
// — AMQP defines "header"/"routing-key"/"exchange"
// — Kafka defines "header"/"topic"/"key"
// Adapters and projects decide. Missing scope or key → "".
type Source interface {
	Lookup(scope, key string) string
	LookupAll(scope, key string) []string
	Body() []byte
	Context() context.Context
}

func FromBytes(b []byte) Source {
	return bytesSource(b)
}

func Into[Req any, PT Parser[Req]](src Source) (*Req, error) {
	t := PT(new(Req))
	if err := t.Parse(src); err != nil {
		return nil, err
	}
	if v, ok := any(t).(Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}
	return t, nil
}

type Parser[Req any] interface {
	*Req
	Parse(Source) error
}

type Validator interface {
	Validate() error
}

type bytesSource []byte

func (b bytesSource) Lookup(_, _ string) string {
	return ""
}

func (b bytesSource) LookupAll(_, _ string) []string {
	return nil
}

func (b bytesSource) Body() []byte {
	return b
}

func (b bytesSource) Context() context.Context {
	return context.Background()
}
