## metactl pages posts create

Create or schedule a Page feed post

```
metactl pages posts create [flags]
```

### Examples

```
  metactl pages posts create --message 'Coming soon' --published=false --scheduled-at 1789900000
```

### Options

```
  -d, --data string        JSON object body
  -f, --file string        read the JSON object body from this explicit path, or - for stdin
  -h, --help               help for create
      --link string        link URL
      --message string     post message
      --published          publish immediately (default true)
      --scheduled-at int   Unix timestamp for scheduled publishing
      --set stringArray    set body field as key=value; repeatable
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

* [metactl pages posts](metactl_pages_posts.md)	 - Manage Page feed posts

