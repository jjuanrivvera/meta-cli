## meta instagram publish reel

Publish a reel with a cover and optional first comment

```
meta instagram publish reel [flags]
```

### Examples

```
  meta instagram publish reel --video ./reel.mp4 --cover-url https://cdn.example/cover.jpg --caption 'Launch' --first-comment 'Details in bio'
```

### Options

```
      --caption string           reel caption
      --cover-url string         public cover image URL
      --first-comment string     comment to create after publishing
  -h, --help                     help for reel
      --poll-attempts int        maximum status checks (default 5)
      --poll-interval duration   container status poll interval (default 1m0s)
      --share-to-feed            also show the reel in the feed (default true)
      --thumb-offset int         thumbnail frame offset in milliseconds
      --video string             local path or public video URL
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

* [meta instagram publish](meta_instagram_publish.md)	 - Run composite Instagram publishing workflows

