package domain

import "strings"

type Role string

const (
	RolePlanner  Role = "planner"
	RoleReviewer Role = "reviewer"
	RoleLeader   Role = "leader"
)

type Principal struct {
	Name string
	Role Role
}

func (p Principal) Validate(required Role) error {
	if strings.TrimSpace(p.Name) == "" {
		return NewError(CodeForbidden, "actor", "操作人不能为空")
	}
	if p.Role != required {
		return NewError(CodeForbidden, "role", "当前角色无权执行该操作")
	}
	return nil
}
