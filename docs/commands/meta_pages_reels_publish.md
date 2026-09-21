## meta pages reels publish

Upload and publish a Page reel in one workflow

```
meta pages reels publish [flags]
```

### Options

```
      --description string              reel description
  -h, --help                            help for publish
      --poll-attempts int               maximum processing status checks (default 60)
      --poll-interval duration          processing status poll interval (default 5s)
      --scheduled-at int                Unix timestamp for scheduled publishing
      --thumbnail-content-type string   thumbnail MIME type; inferred when omitted
      --thumbnail-file string           local custom cover image
      --title string                    reel title
      --video string                    local path or public hosted video URL
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

* [meta pages reels](meta_pages_reels.md)	 - Manage Page reels

