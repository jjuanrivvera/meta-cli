## meta config set

Create or update an account

```
meta config set NAME [flags]
```

### Examples

```
  meta config set work --graph-version v26.0 --page-id 123 --instagram-id 456
```

### Options

```
      --app-id string          Meta app id
      --base-url string        Graph API base URL
      --business-id string     Meta business id
      --graph-version string   Graph API version
  -h, --help                   help for set
      --instagram-id string    Instagram professional account id
      --page-id string         Facebook Page id
      --phone-id string        WhatsApp phone number id
      --rps float              request rate ceiling
      --upload-url string      resumable upload base URL
      --waba-id string         WhatsApp Business Account id
```

### Options inherited from parent commands

```
      --account string   named account to use
      --columns string   comma-separated table or CSV columns
      --dry-run          print equivalent curl requests without sending them
      --filter string    filter result rows with field=value
      --jq string        gojq expression applied before rendering
      --no-color         disable terminal color
  -o, --output string    output format: table, json, yaml, csv, or id (default "table")
      --quiet            suppress non-result messages
      --sort string      sort result rows by field
  -v, --verbose          show request diagnostics
```

### SEE ALSO

* [meta config](meta_config.md)	 - Inspect and edit non-secret account settings

