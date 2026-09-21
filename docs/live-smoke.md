# Live smoke test

These commands are an owner-run checklist. They contact the configured Meta account and can create
or delete content. Start with read-only checks and retain `--dry-run` until every identifier and
request is correct. Use disposable content only.

## Prepare an isolated account

```sh
export META_ACCOUNT=live-smoke
meta config set "$META_ACCOUNT" \
  --graph-version v26.0 \
  --page-id 123456789 \
  --instagram-id 17841400000000000 \
  --business-id 100000000000000 \
  --waba-id 100000000000001 \
  --phone-id 100000000000002 \
  --app-id 100000000000003
meta auth login --account "$META_ACCOUNT"
```

Enter the token at the hidden prompt. If `appsecret_proof` is required, repeat the login with
`--prompt-app-secret`, or set `META_APP_SECRET` for that invocation, and enter only disposable
test credentials. Derive the Page credential before running any `pages` command:

```sh
meta auth pages --account "$META_ACCOUNT" --page-id 123456789 --save
```

## Read-only checks

```sh
meta doctor --account "$META_ACCOUNT" --json
meta auth debug --account "$META_ACCOUNT" -o json
meta pages accounts list --account "$META_ACCOUNT" -o json
meta instagram media list --account "$META_ACCOUNT" --limit 2 -o json
meta instagram limits publishing --account "$META_ACCOUNT" -o json
meta whatsapp phones list --account "$META_ACCOUNT" -o json
meta whatsapp templates list --account "$META_ACCOUNT" --limit 2 -o json
```

## Dry-run writes

Use a future Unix timestamp accepted by the API:

```sh
SCHEDULED_AT=$(date -u -v+1H +%s 2>/dev/null || date -u -d '+1 hour' +%s)
meta pages posts create --account "$META_ACCOUNT" \
  --message "meta disposable smoke post" --published=false \
  --scheduled-at "$SCHEDULED_AT" --dry-run
meta instagram publish reel --account "$META_ACCOUNT" \
  --video ./disposable-smoke.mp4 --caption "meta disposable smoke reel" \
  --cover-url https://cdn.example/disposable-smoke-cover.jpg \
  --first-comment "meta disposable smoke comment" --dry-run
meta whatsapp templates create --account "$META_ACCOUNT" \
  --name meta_disposable_smoke --language en_US --category UTILITY \
  --components '[{"type":"BODY","text":"Disposable smoke {{1}}"}]' --dry-run
```

Only after inspecting those requests, rerun without `--dry-run` and capture each returned ID.
Publishing an Instagram reel is irreversible through the Graph API: there is no media-delete
endpoint, so the disposable reel will remain on the account unless it is removed manually in an
Instagram client. The comment can be deleted through Graph. Verify the created objects, then
delete only the disposable objects for which a delete command exists:

```sh
meta pages posts get DISPOSABLE_POST_ID --account "$META_ACCOUNT" -o json
meta pages posts delete DISPOSABLE_POST_ID --account "$META_ACCOUNT"
meta instagram media get DISPOSABLE_MEDIA_ID --account "$META_ACCOUNT" -o json
meta instagram comments delete DISPOSABLE_COMMENT_ID --account "$META_ACCOUNT"
meta whatsapp templates get DISPOSABLE_TEMPLATE_ID --account "$META_ACCOUNT" -o json
meta whatsapp templates delete --name DISPOSABLE_TEMPLATE_NAME \
  --id DISPOSABLE_TEMPLATE_ID --account "$META_ACCOUNT"
```

If a publish command exits with status `2`, the primary object was created but a follow-up action
failed. Preserve the returned object ID and do not rerun the publish command; retry or clean up
only the named follow-up action.

Do not delete pre-existing content. Finish by removing the local credential:

```sh
meta auth logout --account "$META_ACCOUNT"
```
