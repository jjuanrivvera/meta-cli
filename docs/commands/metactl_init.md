## metactl init

Configure an account and store its credential

```
metactl init [flags]
```

### Examples

```
  metactl init --account work --page-id 123 --instagram-id 456 --waba-id 789 --phone-id 101
```

### Options

```
      --app string             Meta app id
      --app-secret string      optional app secret for appsecret_proof
      --business string        Meta business id
      --graph-url string       Graph API base URL
  -h, --help                   help for init
      --instagram string       Instagram professional account id
      --page string            Facebook Page id
      --phone string           WhatsApp phone number id
      --resumable-url string   resumable upload base URL
      --token string           access token; omit to read it without echo
      --version string         Graph API version
      --waba string            WhatsApp Business Account id
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

