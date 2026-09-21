## meta

Publish and manage Meta business content

### Synopsis

meta manages Instagram publishing, Facebook Pages, and WhatsApp Business through one Graph API client.

### Examples

```
  meta pages posts list --page-id 123
  meta instagram publish reel --instagram-id 456 --video ./reel.mp4 --dry-run
  meta whatsapp send text --phone-id 789 --to 15551234567 --message 'Hello'
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
  -h, --help                   help for meta
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

* [meta agent](meta_agent.md)	 - Generate automation safety policy from command annotations
* [meta alias](meta_alias.md)	 - Manage command aliases
* [meta api](meta_api.md)	 - Send a raw authenticated Graph request
* [meta auth](meta_auth.md)	 - Manage Graph API credentials
* [meta completion](meta_completion.md)	 - Generate shell completion
* [meta config](meta_config.md)	 - Inspect and edit non-secret account settings
* [meta doctor](meta_doctor.md)	 - Check configuration, credentials, and Graph connectivity
* [meta init](meta_init.md)	 - Configure an account and store its credential
* [meta instagram](meta_instagram.md)	 - Publish and manage Instagram professional accounts
* [meta mcp](meta_mcp.md)	 - MCP server management
* [meta pages](meta_pages.md)	 - Publish and manage Facebook Pages
* [meta update](meta_update.md)	 - Install the latest checksum-verified release
* [meta version](meta_version.md)	 - Print build version information
* [meta whatsapp](meta_whatsapp.md)	 - Manage WhatsApp Business Cloud API resources

