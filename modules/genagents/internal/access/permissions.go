// Package access names the permissions declared in module.yaml; a test keeps both in step (rule 51).
package access

const (
	PermissionAccountManage   = "genagents.account.manage"
	PermissionBotRead         = "genagents.bot.read"
	PermissionBotManage       = "genagents.bot.manage"
	PermissionSkillManage     = "genagents.skill.manage"
	PermissionMCPServerManage = "genagents.mcpserver.manage"
	PermissionRunnerManage    = "genagents.runner.manage"
	PermissionRunRead         = "genagents.run.read"
	PermissionRunWrite        = "genagents.run.write"
	PermissionAdminChatUse    = "genagents.adminchat.use"
)

var Permissions = []string{
	PermissionAccountManage, PermissionBotRead, PermissionBotManage, PermissionSkillManage, PermissionMCPServerManage,
	PermissionRunnerManage, PermissionRunRead, PermissionRunWrite, PermissionAdminChatUse,
}
