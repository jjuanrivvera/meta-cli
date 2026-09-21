## meta whatsapp templates create

Create a message template

```
meta whatsapp templates create [flags]
```

### Examples

```
  meta whatsapp templates create --name order_ready --language en_US --category UTILITY --components '[{"type":"BODY","text":"Order {{1}} is ready"}]'
```

### Options

```
      --category string     template category (default "UTILITY")
      --components string   JSON component array
  -d, --data string         JSON object body
  -f, --file string         read the JSON object body from this explicit path, or - for stdin
  -h, --help                help for create
      --language string     template language (default "en_US")
      --name string         template name
      --set stringArray     set body field as key=value; repeatable
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

* [meta whatsapp templates](meta_whatsapp_templates.md)	 - Manage WhatsApp message templates

