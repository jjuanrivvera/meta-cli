## meta instagram containers create

Create an image, video, reel, carousel, or story container

```
meta instagram containers create [flags]
```

### Examples

```
  meta instagram containers create --type reel --url https://cdn.example/reel.mp4 --cover-url https://cdn.example/cover.jpg --caption 'Launch day'
```

### Options

```
      --caption string     post caption
      --children string    comma-separated child container ids
      --cover-url string   public reel cover image URL
  -h, --help               help for create
      --share-to-feed      also show a reel in the feed (default true)
      --thumb-offset int   thumbnail frame offset in milliseconds
      --type string        container type: image, video, reel, story, carousel-item, or carousel (default "image")
      --url string         public image or video URL
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

* [meta instagram containers](meta_instagram_containers.md)	 - Manage Instagram media containers

