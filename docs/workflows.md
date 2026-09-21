# Workflows

## Publish an Instagram reel

Preview every request first:

```sh
meta instagram publish reel --account work \
  --video ./launch.mp4 \
  --cover-url https://cdn.example/cover.jpg \
  --caption "Launch day" \
  --first-comment "Details in bio" \
  --dry-run
```

Remove `--dry-run` only after checking the target account and rendered requests. Local files use
the resumable upload host; public HTTP URLs are handed to Graph for retrieval. The command waits
for the container to finish before publishing and optionally creates the first comment.

## Schedule a Page post

```sh
SCHEDULED_AT=$(date -u -v+1H +%s 2>/dev/null || date -u -d '+1 hour' +%s)
meta pages posts create --account work \
  --message "Coming soon" \
  --published=false \
  --scheduled-at "$SCHEDULED_AT" \
  --dry-run
```

`--scheduled-at` is a Unix timestamp. Keep `--published=false` for a scheduled post.

## Publish a Page video

```sh
meta pages videos publish --account work \
  --file ./launch.mp4 \
  --title "Launch" \
  --description "Product walkthrough" \
  --thumbnail-file ./thumbnail.jpg \
  --dry-run
```

The composite command creates an upload session, transfers the file in chunks, finishes the
session, and uploads the thumbnail. Transient chunk failures can be retried safely by offset.

## Work with WhatsApp templates

```sh
meta whatsapp templates create --account work \
  --name order_ready --language en_US --category UTILITY \
  --components '[{"type":"BODY","text":"Order {{1}} is ready"}]' \
  --dry-run
meta whatsapp templates list --account work --all -o json
```

Sending a message is a real-world side effect and is classified as a write:

```sh
meta whatsapp send text --account work \
  --to 15551234567 --message "Hello" --dry-run
```
