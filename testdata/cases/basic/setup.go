package basic

type Login struct {
	UserID   string   `scan:"path=userId" validate:"required" json:"user_id"`
	Token    string   `scan:"header=Authorization" validate:"required,min=8"`
	Email    string   `validate:"required,email" json:"email"`
	Password string   `validate:"required,min=8,max=64"`
	Tags     []string `scan:"query=tag,repeated" validate:"unique"`
	Page     int      `scan:"query=page" validate:"gte=1,lte=100"`
}
