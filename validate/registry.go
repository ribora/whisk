package validate

// The bool is whether Is<Tag> returns an error worth keeping. To add a built-in,
// drop Is<Tag> in this package and name the tag here.
var BuiltIns = map[string]bool{
	"email":    true,
	"uuid":     true,
	"url":      false,
	"datetime": false,
	"oneof":    false,

	"charset":   true,
	"lowercase": false,
	"uppercase": false,

	"contains":   true,
	"startswith": true,
	"endswith":   true,

	"ip":       true,
	"cidr":     true,
	"hostname": false,
	"fqdn":     false,
	"mac":      true,

	"base64": true,
	"json":   false,

	"phone": false,

	"semver":   false,
	"timezone": true,
	"duration": true,

	"hexcolor": false,

	"currency": true,
	"country":  true,
	"locale":   true,
}
