package toolset

import "github.com/modelcontextprotocol/go-sdk/mcp"

const ServerName = "genwork"

const serverInstructions = `Genwork is a modular, self-hosted workspace platform.
Tool names start with the component that owns them (core_, gentasks_, ...). Only the modules enabled on this
instance and the actions the signed-in user is allowed to perform are listed; the list changes when an
administrator enables or disables a module or changes the user's role.
Every resource is referenced by its opaque token: use the list and get tools to find the tokens other tools need.`

func NewServer(version string) *mcp.Server {
	return mcp.NewServer(
		&mcp.Implementation{Name: ServerName, Title: "Genwork", Version: version},
		&mcp.ServerOptions{Instructions: serverInstructions, HasTools: true},
	)
}
