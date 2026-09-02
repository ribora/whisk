package basic

type Invite struct {
	OrgID string   `scan:"path=orgId" validate:"required" json:"org_id"`
	Token string   `scan:"header=Authorization" validate:"required" json:"token"`
	Email string   `validate:"required,email" json:"email"`
	Name  string   `validate:"required,max=20" json:"name"`
	Tags  []string `scan:"query=tag,repeated" validate:"unique" json:"tags"`
	TTL   int      `scan:"query=ttl" validate:"gte=1,lte=30" json:"ttl"`
}
