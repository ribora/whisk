package scan

import "context"

// Source resolves values from named scopes. Scope is an opaque string
// — HTTP defines "path"/"query"/"header"/"cookie"/"form", AMQP defines
// "header"/"routing-key"/"exchange", Kafka defines "header"/"topic"/"key"
// — adapters and projects decide. Missing scope or key → "".
type Source interface {
	Lookup(scope, key string) string
	LookupAll(scope, key string) []string
	Body() []byte
	Context() context.Context
}
