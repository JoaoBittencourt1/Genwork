import { describe, expect, it } from 'vitest'
import { buildClaudeCodeCommand, buildClientConfigJson, buildMcpUrl, needsReachabilityWarning } from './connection'

describe('assistant connection', () => {
  it('builds the MCP URL from the origin', () => {
    expect(buildMcpUrl('https://genwork.example.com')).toBe('https://genwork.example.com/mcp')
    expect(buildMcpUrl('https://genwork.example.com/')).toBe('https://genwork.example.com/mcp')
  })

  it('builds the Claude Code command', () => {
    expect(buildClaudeCodeCommand('https://genwork.example.com/mcp')).toBe('claude mcp add --transport http genwork https://genwork.example.com/mcp')
  })

  it('builds a configuration other MCP clients understand', () => {
    expect(JSON.parse(buildClientConfigJson('https://genwork.example.com/mcp'))).toEqual({
      mcpServers: { genwork: { type: 'http', url: 'https://genwork.example.com/mcp' } },
    })
  })

  it('warns when claude.ai cannot reach the instance', () => {
    expect(needsReachabilityWarning('http://localhost:8485')).toBe(true)
    expect(needsReachabilityWarning('https://127.0.0.1')).toBe(true)
    expect(needsReachabilityWarning('https://192.168.1.20')).toBe(true)
    expect(needsReachabilityWarning('https://[fd00::1]')).toBe(true)
    expect(needsReachabilityWarning('http://genwork.example.com')).toBe(true)
    expect(needsReachabilityWarning('https://genwork.example.com')).toBe(false)
  })
})
