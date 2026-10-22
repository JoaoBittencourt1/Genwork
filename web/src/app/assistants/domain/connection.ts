import { isReachableFromInternet } from '@/shared/domain/internetReachability'

export const MCP_SERVER_NAME = 'genwork'

// buildMcpUrl is where assistants connect: the gateway routes /mcp of this origin to the MCP server.
export function buildMcpUrl(origin: string): string {
  return `${origin.replace(/\/+$/, '')}/mcp`
}

export function buildClaudeCodeCommand(mcpUrl: string): string {
  return `claude mcp add --transport http ${MCP_SERVER_NAME} ${mcpUrl}`
}

export function buildClientConfigJson(mcpUrl: string): string {
  return JSON.stringify({ mcpServers: { [MCP_SERVER_NAME]: { type: 'http', url: mcpUrl } } }, null, 2)
}

// needsReachabilityWarning is true when claude.ai and Claude Desktop, which connect from the internet, cannot reach
// this origin; Claude Code still works from the user's own machine.
export function needsReachabilityWarning(origin: string): boolean {
  return !isReachableFromInternet(origin)
}
