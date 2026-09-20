## metactl mcp

MCP server management

### Synopsis

Manage MCP servers for AI assistants and code editors

### Options

```
  -h, --help   help for mcp
```

### Options inherited from parent commands

```
      --account string         named account to use
      --app-id string          Meta app id override
      --base-url string        Graph API base URL
      --business-id string     Meta business id override
      --columns string         comma-separated table or CSV columns
      --dry-run                print equivalent curl requests without sending them
      --filter string          filter result rows with field=value
      --graph-version string   Graph API version
      --instagram-id string    Instagram professional account id override
      --jq string              gojq expression applied before rendering
      --no-color               disable terminal color
  -o, --output string          output format: table, json, yaml, csv, or id (default "table")
      --page-id string         Facebook Page id override
      --phone-id string        WhatsApp phone number id override
      --quiet                  suppress non-result messages
      --show-token             show the access token in dry-run output
      --sort string            sort result rows by field
      --upload-url string      resumable upload base URL
  -v, --verbose                show request diagnostics
      --waba-id string         WhatsApp Business Account id override
```

### SEE ALSO

* [metactl](metactl.md)	 - Publish and manage Meta business content
* [metactl mcp claude](metactl_mcp_claude.md)	 - Manage Claude Desktop MCP servers
* [metactl mcp cursor](metactl_mcp_cursor.md)	 - Manage Cursor MCP servers
* [metactl mcp start](metactl_mcp_start.md)	 - Start the MCP server
* [metactl mcp stream](metactl_mcp_stream.md)	 - Stream the MCP server over HTTP
* [metactl mcp tools](metactl_mcp_tools.md)	 - Export tools as JSON
* [metactl mcp vscode](metactl_mcp_vscode.md)	 - Manage VSCode MCP servers

