package custom_format

type Doc struct {
	Title string `toml:"title" validate:"required"`
	Body  string `toml:"body"`
}
