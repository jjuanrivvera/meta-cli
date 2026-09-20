# Live smoke test

These commands are an owner-run checklist. They contact the configured Meta account and can create
or delete content. Start with read-only checks and retain `--dry-run` until every identifier and
request is correct. Use disposable content only.

## Prepare an isolated account

```sh
export METACTL_ACCOUNT=live-smoke
metactl config set "$METACTL_ACCOUNT" \
  --graph-version v26.0 \
  --page-id 123456789 \
  --instagram-id 17841400000000000 \
  --business-id 100000000000000 \
  --waba-id 100000000000001 \
  --phone-id 100000000000002 \
  --app-id 100000000000003
metactl auth login --account "$METACTL_ACCOUNT"
```

Enter the token at the hidden prompt. If `appsecret_proof` is required, repeat the login with
`--prompt-app-secret`, or set `METACTL_APP_SECRET` for that invocation, and enter only disposable
test credentials. Derive the Page credential before running any `pages` command:

```sh
metactl auth pages --account "$METACTL_ACCOUNT" --page-id 123456789 --save
```

## Read-only checks

```sh
metactl doctor --account "$METACTL_ACCOUNT" --json
metactl auth debug --account "$METACTL_ACCOUNT" -o json
metactl pages accounts list --account "$METACTL_ACCOUNT" -o json
metactl instagram media list --account "$METACTL_ACCOUNT" --limit 2 -o json
metactl instagram limits publishing --account "$METACTL_ACCOUNT" -o json
metactl whatsapp phones list --account "$METACTL_ACCOUNT" -o json
metactl whatsapp templates list --account "$METACTL_ACCOUNT" --limit 2 -o json
```

## Dry-run writes

Use a future Unix timestamp accepted by the API:

```sh
SCHEDULED_AT=$(date -u -v+1H +%s 2>/dev/null || date -u -d '+1 hour' +%s)
metactl pages posts create --account "$METACTL_ACCOUNT" \
  --message "metactl disposable smoke post" --published=false \
  --scheduled-at "$SCHEDULED_AT" --dry-run
metactl instagram publish reel --account "$METACTL_ACCOUNT" \
  --video ./disposable-smoke.mp4 --caption "metactl disposable smoke reel" \
  --cover-url https://cdn.example/disposable-smoke-cover.jpg \
  --first-comment "metactl disposable smoke comment" --dry-run
metactl whatsapp templates create --account "$METACTL_ACCOUNT" \
  --name metactl_disposable_smoke --language en_US --category UTILITY \
  --components '[{"type":"BODY","text":"Disposable smoke {{1}}"}]' --dry-run
```

Only after inspecting those requests, rerun without `--dry-run` and capture each returned ID.
Verify the created object, then delete only that same disposable object:

```sh
metactl pages posts get DISPOSABLE_POST_ID --account "$METACTL_ACCOUNT" -o json
metactl pages posts delete DISPOSABLE_POST_ID --account "$METACTL_ACCOUNT"
metactl instagram media get DISPOSABLE_MEDIA_ID --account "$METACTL_ACCOUNT" -o json
metactl instagram comments delete DISPOSABLE_COMMENT_ID --account "$METACTL_ACCOUNT"
metactl whatsapp templates get DISPOSABLE_TEMPLATE_ID --account "$METACTL_ACCOUNT" -o json
metactl whatsapp templates delete --name DISPOSABLE_TEMPLATE_NAME \
  --id DISPOSABLE_TEMPLATE_ID --account "$METACTL_ACCOUNT"
```

Do not delete pre-existing content. Finish by removing the local credential:

```sh
metactl auth logout --account "$METACTL_ACCOUNT"
```
