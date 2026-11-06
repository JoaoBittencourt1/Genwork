export const genagentsQueryKeys = {
  providers: ['genagents', 'providers'] as const,
  accounts: ['genagents', 'accounts'] as const,
  bots: ['genagents', 'bots'] as const,
  skills: ['genagents', 'skills'] as const,
  mcpServers: ['genagents', 'mcp-servers'] as const,
  runners: ['genagents', 'runners'] as const,
  chats: ['genagents', 'chats'] as const,
  chat: (chatToken: string) => ['genagents', 'chat', chatToken] as const,
}
