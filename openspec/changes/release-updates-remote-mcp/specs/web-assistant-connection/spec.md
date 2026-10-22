## Purpose

Gives every member a self-service way to connect an AI assistant to Genwork from the web app, and the consent screen that completes the OAuth sign-in for that assistant.

## ADDED Requirements

### Requirement: Connect an AI assistant page
Every signed-in user SHALL reach a "Connect an AI assistant" page from the sidebar and from a card on the home page. The page SHALL show the instance's MCP URL, which is the current origin followed by `/mcp`, with a copy button. It SHALL give steps per client:
- **Claude (claude.ai and Claude Desktop):** add a custom connector with the URL, then sign in to Genwork when Claude asks.
- **Claude Code:** the command `claude mcp add --transport http genwork <MCP URL>`, with a copy button.
- **Other MCP clients:** the URL and a JSON configuration snippet with the URL.

#### Scenario: Member opens the page
- **WHEN** a member opens the page on `https://genwork.example.com`
- **THEN** the page shows `https://genwork.example.com/mcp` and the Claude Code command with that URL

#### Scenario: Copying the URL
- **WHEN** the user presses the copy button next to the URL
- **THEN** the URL is copied to the clipboard and a confirmation is shown

### Requirement: Reachability warning
The page SHALL warn that claude.ai and Claude Desktop reach Genwork from the internet when the origin is not `https`, or when its host is `localhost`, a loopback address, or a private network address. The warning SHALL say that Claude Code still works from the user's own machine.

#### Scenario: Local development
- **WHEN** the page is opened on `http://localhost:8485`
- **THEN** the warning is shown above the Claude (claude.ai and Desktop) steps

#### Scenario: Public HTTPS instance
- **WHEN** the page is opened on `https://genwork.example.com`
- **THEN** no warning is shown

### Requirement: Consent page
`/oauth/authorize` SHALL show a consent screen for a valid authorization request. The screen SHALL show the client's name, the signed-in user's e-mail, and that the assistant will act as the user with the user's permissions. It SHALL offer "Allow" and "Deny". After either choice, the browser SHALL go to the URL returned by core. A request that core rejects without a redirect (unknown client, unregistered redirect URI) SHALL show an error and stay in Genwork.

#### Scenario: Signed-in user allows
- **WHEN** a signed-in user opens a valid authorization link from Claude and presses "Allow"
- **THEN** the browser goes to Claude's redirect URI with an authorization code

#### Scenario: Invalid client
- **WHEN** the authorization link names an unknown client
- **THEN** the page shows that the connection request is invalid, with no "Allow" button

### Requirement: Login returns to the requested page
When a signed-out user opens a page that requires authentication, the login page SHALL bring them back to that page, with its query string, after they sign in. The return target SHALL be accepted only as a same-origin path starting with a single `/`. Any other value SHALL send the user to the home page.

#### Scenario: Consent while signed out
- **WHEN** a signed-out user opens `/oauth/authorize?client_id=…&state=…` and signs in
- **THEN** they land back on `/oauth/authorize` with the same query string

#### Scenario: Open redirect attempt
- **WHEN** the login page receives `//evil.example.com` or `https://evil.example.com` as the return target
- **THEN** after signing in the user lands on the home page

### Requirement: Localized copy
Every text of the connect and consent pages SHALL come from translation keys in pt-BR and English. Client names and commands are not translated.

#### Scenario: Portuguese interface
- **WHEN** the interface language is pt-BR
- **THEN** the steps, buttons, warning, and consent text are in Portuguese
