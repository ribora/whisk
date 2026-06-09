package custom_validator

type Auth struct {
	Code string `validate:"required,passcode"`
}
