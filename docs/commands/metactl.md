## metactl

Publish and manage Meta business content

### Synopsis

metactl manages Instagram publishing, Facebook Pages, and WhatsApp Business through one Graph API client.

### Examples

```
  metactl pages posts list --page-id 123
  metactl instagram publish reel --instagram-id 456 --video ./reel.mp4 --dry-run
  metactl whatsapp send text --phone-id 789 --to 15551234567 --message 'Hello'
```

### Options

```
      --account string         named account to use
      --app-id string          Meta app id override
      --base-url string        Graph API base URL
      --business-id string     Meta business id override
      --columns string         comma-separated table or CSV columns
      --dry-run                print equivalent curl requests without sending them
      --filter string          filter result rows with field=value
      --graph-version string   Graph API version
  -h, --help                   help for metactl
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

* [metactl agent](metactl_agent.md)	 - Generate automation safety policy from command annotations
* [metactl alias](metactl_alias.md)	 - Manage command aliases
* [metactl api](metactl_api.md)	 - Send a raw authenticated Graph request
* [metactl auth](metactl_auth.md)	 - Manage Graph API credentials
* [metactl completion](metactl_completion.md)	 - Generate shell completion
* [metactl config](metactl_config.md)	 - Inspect and edit non-secret account settings
* [metactl doctor](metactl_doctor.md)	 - Check configuration, credentials, and Graph connectivity
* [metactl init](metactl_init.md)	 - Configure an account and store its credential
* [metactl instagram](metactl_instagram.md)	 - Publish and manage Instagram professional accounts
* [metactl mcp](metactl_mcp.md)	 - MCP server management
* [metactl pages](metactl_pages.md)	 - Publish and manage Facebook Pages
* [metactl update](metactl_update.md)	 - Install the latest checksum-verified release
* [metactl version](metactl_version.md)	 - Print build version information
* [metactl whatsapp](metactl_whatsapp.md)	 - Manage WhatsApp Business Cloud API resources

