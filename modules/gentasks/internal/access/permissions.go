// Package access names the permissions declared in module.yaml; a test keeps both in step (rule 51).
package access

const (
	PermissionBoardRead    = "gentasks.board.read"
	PermissionBoardManage  = "gentasks.board.manage"
	PermissionTaskWrite    = "gentasks.task.write"
	PermissionTaskDelete   = "gentasks.task.delete"
	PermissionGithubManage = "gentasks.github.manage"
	// PermissionRepositoryWrite lets a bot member get a GitHub token for the repositories bots work in; people never do.
	PermissionRepositoryWrite = "gentasks.repository.write"
)

var Permissions = []string{PermissionBoardRead, PermissionBoardManage, PermissionTaskWrite, PermissionTaskDelete, PermissionGithubManage, PermissionRepositoryWrite}
