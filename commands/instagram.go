package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/api"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

func init() {
	registerAPI(func(root *cobra.Command, options *globalOptions) {
		instagram := &cobra.Command{Use: "instagram", Short: "Publish and manage Instagram professional accounts"}
		instagram.AddCommand(
			instagramAccounts(options), instagramContainers(options), instagramPublish(options),
			instagramMedia(options), instagramComments(options), instagramInsights(options), instagramLimits(options),
		)
		root.AddCommand(instagram)
	})
}

func instagramAccounts(options *globalOptions) *cobra.Command {
	return newGroup("accounts", "Discover the Instagram account linked to a Page", nil, options, operationSpec{
		Use: "list", Short: "Get the Instagram account linked to the Page", Kind: kindRead,
		Example: "  meta instagram accounts list --page-id 123 -o json",
		Columns: []string{"id", "name", "instagram_business_account"},
		Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
			pageID, err := requireID(account.PageID, "page-id")
			if err != nil {
				return nil, err
			}
			return client.Read(command.Context(), pageID, url.Values{"fields": {"id,name,instagram_business_account{id,username,name}"}})
		},
	})
}

func instagramContainers(options *globalOptions) *cobra.Command {
	var mediaType, sourceURL, caption, children, coverURL string
	var thumbOffset int
	var shareToFeed bool
	create := operationSpec{
		Use: "create", Short: "Create an image, video, reel, carousel, or story container", Kind: kindWrite,
		Example: "  meta instagram containers create --type reel --url https://cdn.example/reel.mp4 --cover-url https://cdn.example/cover.jpg --caption 'Launch day'",
		Flags: func(command *cobra.Command) {
			command.Flags().StringVar(&mediaType, "type", "image", "container type: image, video, reel, story, carousel-item, or carousel")
			command.Flags().StringVar(&sourceURL, "url", "", "public image or video URL")
			command.Flags().StringVar(&caption, "caption", "", "post caption")
			command.Flags().StringVar(&children, "children", "", "comma-separated child container ids")
			command.Flags().StringVar(&coverURL, "cover-url", "", "public reel cover image URL")
			command.Flags().IntVar(&thumbOffset, "thumb-offset", 0, "thumbnail frame offset in milliseconds")
			command.Flags().BoolVar(&shareToFeed, "share-to-feed", true, "also show a reel in the feed")
		},
		Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
			instagramID, err := requireID(account.InstagramID, "instagram-id")
			if err != nil {
				return nil, err
			}
			body, err := instagramContainerBody(mediaType, sourceURL, caption, children, coverURL, thumbOffset, shareToFeed)
			if err != nil {
				return nil, err
			}
			return client.Write(command.Context(), instagramID+"/media", nil, body)
		},
	}
	status := operationSpec{Use: "status CONTAINER_ID", Short: "Get media container processing status", Kind: kindRead, Args: cobra.ExactArgs(1), Columns: []string{"id", "status_code", "status"}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0], url.Values{"fields": {"id,status,status_code,video_status"}})
	}}
	var uploadFilePath, uploadURL string
	upload := operationSpec{
		Use: "upload CONTAINER_ID", Short: "Upload local or hosted video bytes to a resumable container", Kind: kindWrite, Args: cobra.ExactArgs(1),
		Example: "  meta instagram containers upload 456 --file ./reel.mp4",
		Flags: func(command *cobra.Command) {
			command.Flags().StringVar(&uploadFilePath, "file", "", "local video file")
			command.Flags().StringVar(&uploadURL, "url", "", "public hosted video URL")
		},
		Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
			return uploadInstagramContainer(command, client, args[0], uploadFilePath, uploadURL)
		},
	}
	publish := operationSpec{Use: "publish CONTAINER_ID", Short: "Publish a finished media container", Kind: kindWrite, Args: cobra.ExactArgs(1), Example: "  meta instagram containers publish 456 --instagram-id 123", Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, args []string) (any, error) {
		instagramID, err := requireID(account.InstagramID, "instagram-id")
		if err != nil {
			return nil, err
		}
		body, _ := json.Marshal(map[string]any{"creation_id": args[0]})
		return client.Write(command.Context(), instagramID+"/media_publish", nil, body)
	}}
	return newGroup("containers", "Manage Instagram media containers", nil, options, create, status, upload, publish)
}

func instagramContainerBody(mediaType, sourceURL, caption, children, coverURL string, thumbOffset int, shareToFeed bool) ([]byte, error) {
	body := map[string]any{"caption": caption}
	switch strings.ToLower(mediaType) {
	case "image":
		body["image_url"] = sourceURL
	case "video":
		body["media_type"] = "VIDEO"
		body["video_url"] = sourceURL
	case "reel":
		body["media_type"] = "REELS"
		if sourceURL == "" {
			body["upload_type"] = "resumable"
		} else {
			body["video_url"] = sourceURL
		}
		body["share_to_feed"] = shareToFeed
		if coverURL != "" {
			body["cover_url"] = coverURL
		}
		if thumbOffset > 0 {
			body["thumb_offset"] = thumbOffset
		}
	case "story":
		body["media_type"] = "STORIES"
		if sourceURL == "" {
			body["upload_type"] = "resumable"
		} else {
			body["video_url"] = sourceURL
		}
	case "carousel-item":
		body["is_carousel_item"] = true
		if strings.HasSuffix(strings.ToLower(sourceURL), ".mp4") {
			body["media_type"] = "VIDEO"
			body["video_url"] = sourceURL
		} else {
			body["image_url"] = sourceURL
		}
	case "carousel":
		if children == "" {
			return nil, fmt.Errorf("--children is required for a carousel")
		}
		body["media_type"] = "CAROUSEL"
		body["children"] = splitComma(children)
	default:
		return nil, fmt.Errorf("unsupported container type %q", mediaType)
	}
	if sourceURL == "" && mediaType == "image" {
		return nil, fmt.Errorf("--url is required for an image")
	}
	return json.Marshal(body)
}

func uploadInstagramContainer(command *cobra.Command, client *api.Client, containerID, filePath, hostedURL string) (any, error) {
	if (filePath == "") == (hostedURL == "") {
		return nil, fmt.Errorf("use exactly one of --file or --url")
	}
	if hostedURL != "" {
		headers := make(http.Header)
		headers.Set("file_url", hostedURL)
		return uploadResponse(command, client, api.Request{Method: http.MethodPost, Path: "ig-api-upload/" + containerID, Headers: headers, Upload: true, Idempotent: true})
	}
	offset := int64(0)
	var lastErr error
	for resume := 0; resume < 4; resume++ {
		request, size, err := rawUploadRequest(command.Context(), "ig-api-upload/"+containerID, filePath, offset)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(command.Context(), request)
		if err == nil {
			return decodeResponse(response)
		}
		lastErr = err
		status, statusErr := client.Read(command.Context(), containerID, url.Values{"fields": {"id,status,status_code,video_status"}})
		if statusErr != nil {
			return nil, fmt.Errorf("upload failed: %w; query resume offset: %w", err, statusErr)
		}
		next := uploadOffset(status)
		if next > size {
			return nil, fmt.Errorf("graph reported upload offset %d beyond file size %d", next, size)
		}
		if next == size {
			return map[string]any{"success": true, "bytes_transferred": next, "resumed": true}, nil
		}
		if next <= offset {
			return nil, err
		}
		offset = next
	}
	return nil, fmt.Errorf("resumable upload did not complete: %w", lastErr)
}

func instagramPublish(options *globalOptions) *cobra.Command {
	var video, coverURL, caption, firstComment string
	var thumbOffset, attempts int
	var shareToFeed bool
	var pollInterval time.Duration
	reel := operationSpec{
		Use: "reel", Short: "Publish a reel with a cover and optional first comment", Kind: kindWrite,
		Example: "  meta instagram publish reel --video ./reel.mp4 --cover-url https://cdn.example/cover.jpg --caption 'Launch' --first-comment 'Details in bio'",
		Flags: func(command *cobra.Command) {
			command.Flags().StringVar(&video, "video", "", "local path or public video URL")
			command.Flags().StringVar(&coverURL, "cover-url", "", "public cover image URL")
			command.Flags().StringVar(&caption, "caption", "", "reel caption")
			command.Flags().StringVar(&firstComment, "first-comment", "", "comment to create after publishing")
			command.Flags().IntVar(&thumbOffset, "thumb-offset", 0, "thumbnail frame offset in milliseconds")
			command.Flags().BoolVar(&shareToFeed, "share-to-feed", true, "also show the reel in the feed")
			command.Flags().DurationVar(&pollInterval, "poll-interval", time.Minute, "container status poll interval")
			command.Flags().IntVar(&attempts, "poll-attempts", 5, "maximum status checks")
			_ = command.MarkFlagRequired("video")
		},
		Run: func(command *cobra.Command, current *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
			return publishInstagramReel(command, current, client, account, video, coverURL, caption, firstComment, thumbOffset, shareToFeed, pollInterval, attempts)
		},
	}
	return newGroup("publish", "Run composite Instagram publishing workflows", nil, options, reel)
}

func publishInstagramReel(command *cobra.Command, options *globalOptions, client *api.Client, account config.Account, video, coverURL, caption, firstComment string, thumbOffset int, shareToFeed bool, pollInterval time.Duration, attempts int) (any, error) {
	instagramID, err := requireID(account.InstagramID, "instagram-id")
	if err != nil {
		return nil, err
	}
	isURL := strings.HasPrefix(video, "https://") || strings.HasPrefix(video, "http://")
	createBody, err := instagramContainerBody("reel", func() string {
		if isURL {
			return video
		}
		return ""
	}(), caption, "", coverURL, thumbOffset, shareToFeed)
	if err != nil {
		return nil, err
	}
	created, err := client.Write(command.Context(), instagramID+"/media", nil, createBody)
	if err != nil {
		return nil, err
	}
	containerID := stringField(created, "id")
	if containerID == "" && options.dryRun {
		containerID = "CONTAINER_ID"
	}
	if containerID == "" {
		return nil, fmt.Errorf("container response did not include id")
	}
	if !isURL {
		if _, err := uploadInstagramContainer(command, client, containerID, video, ""); err != nil {
			return nil, err
		}
	}
	if _, err := waitForInstagramContainer(command, options, client, containerID, pollInterval, attempts); err != nil {
		return nil, err
	}
	publishBody, _ := json.Marshal(map[string]any{"creation_id": containerID})
	published, err := client.Write(command.Context(), instagramID+"/media_publish", nil, publishBody)
	if err != nil {
		return nil, err
	}
	mediaID := stringField(published, "id")
	if mediaID == "" && options.dryRun {
		mediaID = "PUBLISHED_MEDIA_ID"
	}
	if firstComment != "" {
		commentBody, _ := json.Marshal(map[string]any{"message": firstComment})
		if _, err := client.Write(command.Context(), mediaID+"/comments", nil, commentBody); err != nil {
			result := published
			if object, ok := published.(map[string]any); ok {
				object["first_comment_status"] = "failed"
				object["first_comment_error"] = err.Error()
			} else {
				result = map[string]any{"id": mediaID, "published": published, "first_comment_status": "failed", "first_comment_error": err.Error()}
			}
			fmt.Fprintf(command.ErrOrStderr(), "warning: reel %s was published, but its first comment failed\n", mediaID)
			return result, &partialFailureError{message: fmt.Sprintf("reel %s was published but the first comment failed: %v", mediaID, err)}
		}
	}
	return published, nil
}

func waitForInstagramContainer(command *cobra.Command, options *globalOptions, client *api.Client, containerID string, interval time.Duration, attempts int) (any, error) {
	for attempt := 0; attempt < attempts; attempt++ {
		status, err := client.Read(command.Context(), containerID, url.Values{"fields": {"id,status,status_code,video_status"}})
		if err != nil {
			return nil, err
		}
		if options.dryRun {
			return status, nil
		}
		code := strings.ToUpper(stringField(status, "status_code"))
		switch code {
		case "FINISHED":
			return status, nil
		case "PUBLISHED":
			return nil, fmt.Errorf("container %s is already PUBLISHED; refusing a duplicate publish", containerID)
		case "ERROR", "EXPIRED":
			return nil, fmt.Errorf("container %s entered %s status", containerID, code)
		}
		select {
		case <-command.Context().Done():
			return nil, command.Context().Err()
		case <-time.After(interval):
		}
	}
	return nil, fmt.Errorf("container %s did not finish after %d checks", containerID, attempts)
}

func instagramMedia(options *globalOptions) *cobra.Command {
	list := operationSpec{Use: "list", Short: "List published media", Kind: kindRead, Columns: []string{"id", "media_type", "caption", "timestamp", "permalink"}, Flags: listFlags, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		instagramID, err := requireID(account.InstagramID, "instagram-id")
		if err != nil {
			return nil, err
		}
		all, limit, after, fields := listValues(command)
		query := url.Values{}
		if after != "" {
			query.Set("after", after)
		}
		if fields == "" {
			fields = "id,media_type,caption,timestamp,permalink"
		}
		query.Set("fields", fields)
		return client.List(command.Context(), instagramID+"/media", query, all, limit)
	}}
	get := operationSpec{Use: "get MEDIA_ID", Short: "Get published media", Kind: kindRead, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0], url.Values{"fields": {"id,media_type,caption,timestamp,permalink,media_url,thumbnail_url"}})
	}}
	return newGroup("media", "Inspect published Instagram media", nil, options, list, get)
}

func instagramComments(options *globalOptions) *cobra.Command {
	list := operationSpec{Use: "list MEDIA_ID", Short: "List media comments", Kind: kindRead, Args: cobra.ExactArgs(1), Flags: listFlags, Columns: []string{"id", "username", "text", "timestamp", "hidden"}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		all, limit, after, fields := listValues(command)
		query := url.Values{"fields": {fields}}
		if fields == "" {
			query.Set("fields", "id,username,text,timestamp,hidden")
		}
		if after != "" {
			query.Set("after", after)
		}
		return client.List(command.Context(), args[0]+"/comments", query, all, limit)
	}}
	create, createMessage := instagramCommentWrite("create MEDIA_ID", "Create a comment on media", "comments", options)
	replies := operationSpec{Use: "replies COMMENT_ID", Short: "List replies to a comment", Kind: kindRead, Args: cobra.ExactArgs(1), Flags: listFlags, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		all, limit, after, _ := listValues(command)
		query := url.Values{}
		if after != "" {
			query.Set("after", after)
		}
		return client.List(command.Context(), args[0]+"/replies", query, all, limit)
	}}
	reply, replyMessage := instagramCommentWrite("reply COMMENT_ID", "Reply to a comment", "replies", options)
	_ = createMessage
	_ = replyMessage
	var hidden bool
	hide := operationSpec{Use: "hide COMMENT_ID", Short: "Hide or unhide a comment", Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().BoolVar(&hidden, "hidden", true, "hide when true; unhide when false")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		body, _ := json.Marshal(map[string]any{"hide": hidden})
		return client.Write(command.Context(), args[0], nil, body)
	}}
	remove := operationSpec{Use: "delete COMMENT_ID", Short: "Delete a comment", Kind: kindDestructive, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Remove(command.Context(), args[0], nil)
	}}
	return newGroup("comments", "Moderate Instagram comments", nil, options, list, create, replies, reply, hide, remove)
}

func instagramCommentWrite(use, short, edge string, _ *globalOptions) (operationSpec, *string) {
	message := new(string)
	return operationSpec{Use: use, Short: short, Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(message, "message", "", "comment text")
		_ = command.MarkFlagRequired("message")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		body, _ := json.Marshal(map[string]any{"message": *message})
		return client.Write(command.Context(), args[0]+"/"+edge, nil, body)
	}}, message
}

func instagramInsights(options *globalOptions) *cobra.Command {
	var accountMetrics, mediaMetrics, period string
	accountSpec := operationSpec{Use: "account", Short: "Get Instagram account insights", Kind: kindRead, Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&accountMetrics, "metrics", "", "comma-separated metrics supported by the configured Graph version")
		command.Flags().StringVar(&period, "period", "day", "metric period")
		_ = command.MarkFlagRequired("metrics")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		id, err := requireID(account.InstagramID, "instagram-id")
		if err != nil {
			return nil, err
		}
		return client.Read(command.Context(), id+"/insights", url.Values{"metric": {accountMetrics}, "period": {period}})
	}}
	mediaSpec := operationSpec{Use: "media MEDIA_ID", Short: "Get Instagram media insights", Kind: kindRead, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&mediaMetrics, "metrics", "", "comma-separated metrics supported by the configured Graph version")
		_ = command.MarkFlagRequired("metrics")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0]+"/insights", url.Values{"metric": {mediaMetrics}})
	}}
	return newGroup("insights", "Read Instagram account and media insights", nil, options, accountSpec, mediaSpec)
}

func instagramLimits(options *globalOptions) *cobra.Command {
	return newGroup("limits", "Inspect Instagram publishing limits", nil, options, operationSpec{Use: "publishing", Short: "Get content publishing usage", Kind: kindRead, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		id, err := requireID(account.InstagramID, "instagram-id")
		if err != nil {
			return nil, err
		}
		return client.Read(command.Context(), id+"/content_publishing_limit", url.Values{"fields": {"quota_usage,config"}})
	}})
}
