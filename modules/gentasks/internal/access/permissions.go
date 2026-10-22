// Package access names the permissions declared in module.yaml; a test keeps both in step (rule 51).
package access

const (
	PermissionBoardRead    = "gentasks.board.read"
	PermissionBoardManage  = "gentasks.board.manage"
	PermissionTaskWrite    = "gentasks.task.write"
	PermissionTaskDelete   = "gentasks.task.delete"
	PermissionGithubManage = "gentasks.github.manage"
)

var Permissions = []string{PermissionBoardRead, PermissionBoardManage, PermissionTaskWrite, PermissionTaskDelete, PermissionGithubManage}
