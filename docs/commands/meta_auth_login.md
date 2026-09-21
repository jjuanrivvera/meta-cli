## meta auth login

Verify and store an access token

```
meta auth login [flags]
```

### Examples

```
  meta auth login --account work
  printf '%s\n' "$TOKEN" | meta auth login --account ci
```

### Options

```
  -h, --help                help for login
      --prompt-app-secret   prompt without echo for an app secret to store
      --token string        access token; omit to read it without echo
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
      --sort string            sort result rows by field
      --upload-url string      resumable upload base URL
  -v, --verbose                show request diagnostics
      --waba-id string         WhatsApp Business Account id override
```

### SEE ALSO

* [meta auth](meta_auth.md)	 - Manage Graph API credentials

