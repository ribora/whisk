package err_bad_tag

type Bad struct {
	X string `validate:"required,bogusvalidator"`
}
