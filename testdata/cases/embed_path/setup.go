package embed_path

type RoleRequest struct {
	Name string `json:"name" validate:"required"`
}

type UpdateRoleRequest struct {
	Role RoleRequest `scan:"embed"`
	ID   int64       `scan:"path=roleId"`
}
