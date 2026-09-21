## meta auth

Manage Graph API credentials

### Options

```
  -h, --help   help for auth
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

* [meta](meta.md)	 - Publish and manage Meta business content
* [meta auth debug](meta_auth_debug.md)	 - Inspect the active token with debug_token
* [meta auth exchange](meta_auth_exchange.md)	 - Exchange the active user token for a long-lived token
* [meta auth login](meta_auth_login.md)	 - Verify and store an access token
* [meta auth logout](meta_auth_logout.md)	 - Delete the stored credential
* [meta auth pages](meta_auth_pages.md)	 - List Pages and derived Page access tokens
* [meta auth status](meta_auth_status.md)	 - Verify the active identity

