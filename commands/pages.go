package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/api"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

func init() {
	registerAPI(func(root *cobra.Command, options *globalOptions) {
		pages := &cobra.Command{Use: "pages", Short: "Publish and manage Facebook Pages"}
		pages.AddCommand(
			pageAccounts(options), pagePosts(options), pagePhotos(options), pageVideos(options),
			pageReels(options), pageComments(options), pageInsights(options),
		)
		root.AddCommand(pages)
	})
}

func pageAccounts(options *globalOptions) *cobra.Command {
	return newGroup("accounts", "List Pages and derived Page tokens", nil, options, operationSpec{
		Use: "list", Short: "List Pages available to the active user token", Kind: kindRead,
		Example: "  meta pages accounts list --all -o json", Flags: listFlags,
		Columns: []string{"id", "name", "category", "tasks"},
		Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, _ []string) (any, error) {
			all, limit, after, fields := listValues(command)
			if fields == "" {
				fields = "id,name,category,tasks"
			}
			query := url.Values{"fields": {fields}}
			if after != "" {
				query.Set("after", after)
			}
			return client.List(command.Context(), "me/accounts", query, all, limit)
		},
	})
}

func pagePosts(options *globalOptions) *cobra.Command {
	var createBody, updateBody writeOptions
	var message, link string
	var published bool
	var scheduled int64
	create := operationSpec{
		Use: "create", Short: "Create or schedule a Page feed post", Kind: kindWrite,
		Example: "  meta pages posts create --message 'Coming soon' --published=false --scheduled-at 1789900000",
		Flags: func(command *cobra.Command) {
			command.Flags().StringVar(&message, "message", "", "post message")
			command.Flags().StringVar(&link, "link", "", "link URL")
			command.Flags().BoolVar(&published, "published", true, "publish immediately")
			command.Flags().Int64Var(&scheduled, "scheduled-at", 0, "Unix timestamp for scheduled publishing")
			createBody.addFlags(command)
		},
		Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
			pageID, err := requireID(account.PageID, "page-id")
			if err != nil {
				return nil, err
			}
			defaults := map[string]any{"message": message, "link": link, "published": published}
			if scheduled > 0 {
				defaults["published"] = false
				defaults["scheduled_publish_time"] = scheduled
			}
			body, err := createBody.body(command, defaults)
			if err != nil {
				return nil, err
			}
			return client.Write(command.Context(), pageID+"/feed", nil, body)
		},
	}
	list := operationSpec{Use: "list", Short: "List Page feed posts", Kind: kindRead, Flags: listFlags, Columns: []string{"id", "message", "created_time", "is_published", "scheduled_publish_time"}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		all, limit, after, fields := listValues(command)
		if fields == "" {
			fields = "id,message,created_time,is_published,scheduled_publish_time,permalink_url"
		}
		query := url.Values{"fields": {fields}}
		if after != "" {
			query.Set("after", after)
		}
		return client.List(command.Context(), pageID+"/feed", query, all, limit)
	}}
	get := operationSpec{Use: "get POST_ID", Short: "Get a Page post", Kind: kindRead, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0], url.Values{"fields": {"id,message,created_time,is_published,scheduled_publish_time,permalink_url"}})
	}}
	var updateMessage string
	update := operationSpec{Use: "update POST_ID", Short: "Update a Page post", Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&updateMessage, "message", "", "replacement message")
		updateBody.addFlags(command)
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		body, err := updateBody.body(command, map[string]any{"message": updateMessage})
		if err != nil {
			return nil, err
		}
		return client.Write(command.Context(), args[0], nil, body)
	}}
	remove := operationSpec{Use: "delete POST_ID", Short: "Delete a Page post", Kind: kindDestructive, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Remove(command.Context(), args[0], nil)
	}}
	return newGroup("posts", "Manage Page feed posts", []string{"post"}, options, create, list, get, update, remove)
}

func pagePhotos(options *globalOptions) *cobra.Command {
	var sourceURL, caption string
	create := operationSpec{Use: "create", Short: "Publish a photo from a public URL", Kind: kindWrite, Example: "  meta pages photos create --url https://cdn.example/photo.jpg --caption 'Launch'", Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&sourceURL, "url", "", "public photo URL")
		command.Flags().StringVar(&caption, "caption", "", "photo caption")
		_ = command.MarkFlagRequired("url")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		body, _ := json.Marshal(map[string]any{"url": sourceURL, "caption": caption})
		return client.Write(command.Context(), pageID+"/photos", nil, body)
	}}
	list := operationSpec{Use: "list", Short: "List Page photos", Kind: kindRead, Flags: listFlags, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		all, limit, after, fields := listValues(command)
		if fields == "" {
			fields = "id,name,created_time,picture,images"
		}
		query := url.Values{"fields": {fields}}
		if after != "" {
			query.Set("after", after)
		}
		return client.List(command.Context(), pageID+"/photos", query, all, limit)
	}}
	get := operationSpec{Use: "get PHOTO_ID", Short: "Get a Page photo", Kind: kindRead, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0], url.Values{"fields": {"id,name,created_time,picture,images"}})
	}}
	return newGroup("photos", "Manage Page photos", []string{"photo"}, options, create, list, get)
}

func pageVideos(options *globalOptions) *cobra.Command {
	var fileSize int64
	start := operationSpec{Use: "start", Short: "Start a resumable Page video upload", Kind: kindWrite, Flags: func(command *cobra.Command) {
		command.Flags().Int64Var(&fileSize, "file-size", 0, "video size in bytes")
		_ = command.MarkFlagRequired("file-size")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		return client.Write(command.Context(), pageID+"/videos", url.Values{"upload_phase": {"start"}, "file_size": {strconv.FormatInt(fileSize, 10)}}, nil)
	}}
	var uploadPath, startOffset string
	upload := operationSpec{Use: "upload SESSION_ID", Short: "Upload an idempotent video chunk", Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&uploadPath, "file", "", "video chunk file")
		command.Flags().StringVar(&startOffset, "start-offset", "0", "chunk start offset")
		_ = command.MarkFlagRequired("file")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, args []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		return uploadPageVideoChunk(command, client, pageID, args[0], startOffset, uploadPath)
	}}
	status := operationSpec{Use: "status VIDEO_ID", Short: "Get Page video processing status", Kind: kindRead, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0], url.Values{"fields": {"id,status"}})
	}}
	var finishSession, title, description string
	var scheduledAt int64
	finish := operationSpec{Use: "finish", Short: "Finish and publish a resumable Page video", Kind: kindWrite, Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&finishSession, "session-id", "", "upload session id")
		command.Flags().StringVar(&title, "title", "", "video title")
		command.Flags().StringVar(&description, "description", "", "video description")
		command.Flags().Int64Var(&scheduledAt, "scheduled-at", 0, "Unix timestamp for scheduled publishing")
		_ = command.MarkFlagRequired("session-id")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		query := url.Values{"upload_phase": {"finish"}, "upload_session_id": {finishSession}, "title": {title}, "description": {description}}
		if scheduledAt > 0 {
			query.Set("published", "false")
			query.Set("scheduled_publish_time", strconv.FormatInt(scheduledAt, 10))
		}
		return client.Write(command.Context(), pageID+"/videos", query, nil)
	}}
	var thumbnailFile, thumbnailType string
	var preferred bool
	thumbnail := operationSpec{Use: "thumbnail VIDEO_ID", Short: "Set the preferred video thumbnail", Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&thumbnailFile, "file", "", "local thumbnail image file")
		command.Flags().StringVar(&thumbnailType, "content-type", "", "image MIME type; inferred from the file when omitted")
		command.Flags().BoolVar(&preferred, "preferred", true, "make this the preferred thumbnail")
		_ = command.MarkFlagRequired("file")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return setPageVideoThumbnail(command, client, args[0], thumbnailFile, thumbnailType, preferred)
	}}
	var publishFile, publishTitle, publishDescription, publishThumbnail, publishThumbnailType string
	var publishScheduled int64
	publish := operationSpec{Use: "publish", Short: "Upload and publish a Page video in one workflow", Kind: kindWrite, Example: "  meta pages videos publish --file ./video.mp4 --title 'Launch' --thumbnail-file ./thumb.jpg", Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&publishFile, "file", "", "local video file")
		command.Flags().StringVar(&publishTitle, "title", "", "video title")
		command.Flags().StringVar(&publishDescription, "description", "", "video description")
		command.Flags().StringVar(&publishThumbnail, "thumbnail-file", "", "local thumbnail image file")
		command.Flags().StringVar(&publishThumbnailType, "thumbnail-content-type", "", "thumbnail MIME type; inferred when omitted")
		command.Flags().Int64Var(&publishScheduled, "scheduled-at", 0, "Unix timestamp for scheduled publishing")
		_ = command.MarkFlagRequired("file")
	}, Run: func(command *cobra.Command, current *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		return publishPageVideo(command, current, client, account, publishFile, publishTitle, publishDescription, publishThumbnail, publishThumbnailType, publishScheduled)
	}}
	return newGroup("videos", "Manage resumable Page video uploads", []string{"video"}, options, start, upload, status, finish, thumbnail, publish)
}

func uploadPageVideoChunk(command *cobra.Command, client *api.Client, pageID, sessionID, offset, filePath string) (any, error) {
	start, err := strconv.ParseInt(offset, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid start offset %q", offset)
	}
	info, err := uploadFileInfo(command.Context(), filePath)
	if err != nil {
		return nil, err
	}
	return uploadPageVideoRange(command, client, pageID, sessionID, start, filePath, 0, info.Size())
}

func uploadPageVideoRange(command *cobra.Command, client *api.Client, pageID, sessionID string, start int64, filePath string, sourceOffset, length int64) (any, error) {
	bodyFactory, contentType, err := multipartFileBody(command.Context(), filePath, "video_file_chunk", "application/octet-stream", nil, sourceOffset, length)
	if err != nil {
		return nil, err
	}
	query := url.Values{"upload_phase": {"transfer"}, "upload_session_id": {sessionID}, "start_offset": {strconv.FormatInt(start, 10)}}
	return uploadResponse(command, client, api.Request{
		Method: http.MethodPost, Path: pageID + "/videos", Query: query,
		BodyFactory: bodyFactory, MultipartForm: multipartDryRun(filePath, "video_file_chunk", "application/octet-stream", nil, sourceOffset, length),
		Headers: http.Header{"Content-Type": {contentType}}, Idempotent: true, LongRunning: true,
	})
}

func publishPageVideo(command *cobra.Command, options *globalOptions, client *api.Client, account config.Account, filePath, title, description, thumbnailFile, thumbnailType string, scheduledAt int64) (any, error) {
	pageID, err := requireID(account.PageID, "page-id")
	if err != nil {
		return nil, err
	}
	info, err := uploadFileInfo(command.Context(), filePath)
	if err != nil {
		return nil, fmt.Errorf("stat video: %w", err)
	}
	started, err := client.Write(command.Context(), pageID+"/videos", url.Values{"upload_phase": {"start"}, "file_size": {strconv.FormatInt(info.Size(), 10)}}, nil)
	if err != nil {
		return nil, err
	}
	sessionID, videoID := stringField(started, "upload_session_id"), stringField(started, "video_id")
	if options.dryRun {
		if sessionID == "" {
			sessionID = "UPLOAD_SESSION_ID"
		}
		if videoID == "" {
			videoID = "VIDEO_ID"
		}
	}
	if sessionID == "" || videoID == "" {
		return nil, fmt.Errorf("upload start response omitted session or video id")
	}
	startOffset := integerValueField(started, "start_offset")
	endOffset := integerValueField(started, "end_offset")
	if options.dryRun {
		startOffset, endOffset = 0, info.Size()
	}
	if startOffset < 0 || startOffset > info.Size() {
		return nil, fmt.Errorf("invalid upload start offset %d for file size %d", startOffset, info.Size())
	}
	for startOffset < info.Size() {
		if endOffset <= startOffset || endOffset > info.Size() {
			return nil, fmt.Errorf("invalid upload offsets %d..%d for file size %d", startOffset, endOffset, info.Size())
		}
		transferred, err := uploadPageVideoRange(command, client, pageID, sessionID, startOffset, filePath, startOffset, endOffset-startOffset)
		if err != nil {
			return nil, err
		}
		if options.dryRun {
			startOffset = endOffset
			continue
		}
		nextStart := integerValueField(transferred, "start_offset")
		nextEnd := integerValueField(transferred, "end_offset")
		if nextStart <= startOffset {
			return nil, fmt.Errorf("upload transfer did not advance beyond offset %d", startOffset)
		}
		if nextStart > info.Size() || nextEnd < nextStart || nextEnd > info.Size() {
			return nil, fmt.Errorf("invalid upload offsets %d..%d for file size %d", nextStart, nextEnd, info.Size())
		}
		startOffset, endOffset = nextStart, nextEnd
	}
	query := url.Values{"upload_phase": {"finish"}, "upload_session_id": {sessionID}, "title": {title}, "description": {description}}
	if scheduledAt > 0 {
		query.Set("published", "false")
		query.Set("scheduled_publish_time", strconv.FormatInt(scheduledAt, 10))
	}
	finished, err := client.Write(command.Context(), pageID+"/videos", query, nil)
	if err != nil {
		return nil, err
	}
	if thumbnailFile != "" {
		if _, err := setPageVideoThumbnail(command, client, videoID, thumbnailFile, thumbnailType, true); err != nil {
			result := map[string]any{"id": videoID, "published": finished, "thumbnail_status": "failed", "thumbnail_error": err.Error()}
			fmt.Fprintf(command.ErrOrStderr(), "warning: video %s was published, but its thumbnail upload failed\n", videoID)
			return result, &partialFailureError{message: fmt.Sprintf("video %s was published but the thumbnail upload failed: %v", videoID, err)}
		}
	}
	return resultWithID(videoID, finished), nil
}

func setPageVideoThumbnail(command *cobra.Command, client *api.Client, videoID, filePath, contentType string, preferred bool) (any, error) {
	bodyFactory, multipartType, err := multipartFileBody(command.Context(), filePath, "source", contentType, map[string]string{"is_preferred": strconv.FormatBool(preferred)}, 0, 0)
	if err != nil {
		return nil, err
	}
	return uploadResponse(command, client, api.Request{
		Method: http.MethodPost, Path: videoID + "/thumbnails", BodyFactory: bodyFactory,
		MultipartForm: multipartDryRun(filePath, "source", contentType, map[string]string{"is_preferred": strconv.FormatBool(preferred)}, 0, 0),
		Headers:       http.Header{"Content-Type": {multipartType}},
		Idempotent:    true, LongRunning: true,
	})
}

func integerValueField(value any, name string) int64 {
	object, ok := value.(map[string]any)
	if !ok {
		return 0
	}
	return integerValue(object[name])
}

func pageReels(options *globalOptions) *cobra.Command {
	start := operationSpec{Use: "start", Short: "Start a Page reel upload", Kind: kindWrite, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		body, _ := json.Marshal(map[string]any{"upload_phase": "start"})
		return client.Write(command.Context(), pageID+"/video_reels", nil, body)
	}}
	var uploadFile, uploadURL string
	upload := operationSpec{Use: "upload VIDEO_ID", Short: "Upload local or hosted Page reel video", Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&uploadFile, "file", "", "local video file")
		command.Flags().StringVar(&uploadURL, "url", "", "public hosted video URL")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return uploadPageReel(command, client, args[0], uploadFile, uploadURL)
	}}
	status := operationSpec{Use: "status VIDEO_ID", Short: "Get Page reel processing status", Kind: kindRead, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0], url.Values{"fields": {"id,status"}})
	}}
	var finishVideo, description, finishTitle string
	var finishScheduled int64
	finish := operationSpec{Use: "finish", Short: "Publish a finished Page reel", Kind: kindWrite, Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&finishVideo, "video-id", "", "uploaded video id")
		command.Flags().StringVar(&description, "description", "", "reel description")
		command.Flags().StringVar(&finishTitle, "title", "", "reel title")
		command.Flags().Int64Var(&finishScheduled, "scheduled-at", 0, "Unix timestamp for scheduled publishing")
		_ = command.MarkFlagRequired("video-id")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		body := pageReelFinishBody(finishVideo, finishTitle, description, finishScheduled)
		return client.Write(command.Context(), pageID+"/video_reels", nil, body)
	}}
	var publishVideo, publishDescription, publishTitle, publishThumbnail, publishThumbnailType string
	var publishScheduled int64
	var pollInterval time.Duration
	var pollAttempts int
	publish := operationSpec{Use: "publish", Short: "Upload and publish a Page reel in one workflow", Kind: kindWrite, Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&publishVideo, "video", "", "local path or public hosted video URL")
		command.Flags().StringVar(&publishDescription, "description", "", "reel description")
		command.Flags().StringVar(&publishTitle, "title", "", "reel title")
		command.Flags().Int64Var(&publishScheduled, "scheduled-at", 0, "Unix timestamp for scheduled publishing")
		command.Flags().StringVar(&publishThumbnail, "thumbnail-file", "", "local custom cover image")
		command.Flags().StringVar(&publishThumbnailType, "thumbnail-content-type", "", "thumbnail MIME type; inferred when omitted")
		command.Flags().DurationVar(&pollInterval, "poll-interval", 5*time.Second, "processing status poll interval")
		command.Flags().IntVar(&pollAttempts, "poll-attempts", 60, "maximum processing status checks")
		_ = command.MarkFlagRequired("video")
	}, Run: func(command *cobra.Command, current *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		return publishPageReel(command, current, client, account, publishVideo, publishTitle, publishDescription, publishThumbnail, publishThumbnailType, publishScheduled, pollInterval, pollAttempts)
	}}
	return newGroup("reels", "Manage Page reels", []string{"reel"}, options, start, upload, status, finish, publish)
}

func uploadPageReel(command *cobra.Command, client *api.Client, videoID, filePath, hostedURL string) (any, error) {
	if (filePath == "") == (hostedURL == "") {
		return nil, fmt.Errorf("use exactly one of --file or --url")
	}
	headers := make(http.Header)
	if hostedURL != "" {
		headers.Set("file_url", hostedURL)
		return uploadResponse(command, client, api.Request{Method: http.MethodPost, Path: "video-upload/" + videoID, Headers: headers, Upload: true, Idempotent: true})
	}
	offset := int64(0)
	var lastErr error
	for resume := 0; resume < 4; resume++ {
		request, size, err := rawUploadRequest(command.Context(), "video-upload/"+videoID, filePath, offset)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(command.Context(), request)
		if err == nil {
			return decodeResponse(response)
		}
		lastErr = err
		status, statusErr := client.Read(command.Context(), videoID, url.Values{"fields": {"id,status"}})
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

func publishPageReel(command *cobra.Command, options *globalOptions, client *api.Client, account config.Account, video, title, description, thumbnailFile, thumbnailType string, scheduledAt int64, pollInterval time.Duration, pollAttempts int) (any, error) {
	pageID, err := requireID(account.PageID, "page-id")
	if err != nil {
		return nil, err
	}
	started, err := client.Write(command.Context(), pageID+"/video_reels", nil, []byte(`{"upload_phase":"start"}`))
	if err != nil {
		return nil, err
	}
	videoID := stringField(started, "video_id")
	if videoID == "" && options.dryRun {
		videoID = "VIDEO_ID"
	}
	if videoID == "" {
		return nil, fmt.Errorf("reel start response omitted video id")
	}
	if strings.HasPrefix(video, "http://") || strings.HasPrefix(video, "https://") {
		_, err = uploadPageReel(command, client, videoID, "", video)
	} else {
		_, err = uploadPageReel(command, client, videoID, video, "")
	}
	if err != nil {
		return nil, err
	}
	finished, err := client.Write(command.Context(), pageID+"/video_reels", nil, pageReelFinishBody(videoID, title, description, scheduledAt))
	if err != nil {
		return nil, err
	}
	if !options.dryRun {
		if _, err := waitForPageReel(command, client, videoID, pollInterval, pollAttempts); err != nil {
			return map[string]any{"id": videoID, "published": finished, "processing_status": "failed", "processing_error": err.Error()}, &partialFailureError{message: fmt.Sprintf("reel %s was published but processing did not complete: %v", videoID, err)}
		}
	}
	if thumbnailFile != "" {
		if _, err := setPageVideoThumbnail(command, client, videoID, thumbnailFile, thumbnailType, true); err != nil {
			return map[string]any{"id": videoID, "published": finished, "thumbnail_status": "failed", "thumbnail_error": err.Error()}, &partialFailureError{message: fmt.Sprintf("reel %s was published but the thumbnail upload failed: %v", videoID, err)}
		}
	}
	return resultWithID(videoID, finished), nil
}

func resultWithID(id string, value any) map[string]any {
	result := map[string]any{}
	if object, ok := value.(map[string]any); ok {
		for key, item := range object {
			result[key] = item
		}
	} else if value != nil {
		result["result"] = value
	}
	result["id"] = id
	return result
}

func pageReelFinishBody(videoID, title, description string, scheduledAt int64) []byte {
	value := map[string]any{"upload_phase": "finish", "video_id": videoID, "video_state": "PUBLISHED", "description": description, "title": title}
	if scheduledAt > 0 {
		value["video_state"] = "SCHEDULED"
		value["scheduled_publish_time"] = scheduledAt
	}
	body, _ := json.Marshal(value)
	return body
}

func waitForPageReel(command *cobra.Command, client *api.Client, videoID string, interval time.Duration, attempts int) (any, error) {
	for attempt := 0; attempt < attempts; attempt++ {
		status, err := client.Read(command.Context(), videoID, url.Values{"fields": {"id,status"}})
		if err != nil {
			return nil, err
		}
		state := strings.ToLower(statusState(status))
		switch state {
		case "complete", "completed", "ready", "published":
			return status, nil
		case "error", "failed", "expired":
			return nil, fmt.Errorf("reel %s entered %s processing status", videoID, state)
		}
		select {
		case <-command.Context().Done():
			return nil, command.Context().Err()
		case <-time.After(interval):
		}
	}
	return nil, fmt.Errorf("reel %s processing did not complete after %d checks", videoID, attempts)
}

func statusState(value any) string {
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	sources := []map[string]any{object}
	if nested, ok := object["status"].(map[string]any); ok {
		sources = append(sources, nested)
	}

	// A terminal phase failure is authoritative even when an earlier phase is complete.
	for _, source := range sources {
		for _, phase := range []string{"uploading_phase", "processing_phase", "publishing_phase"} {
			if state := nestedStatus(source[phase]); isFailedState(state) {
				return state
			}
		}
	}
	for _, source := range sources {
		if state := nestedStatus(source["publishing_phase"]); state != "" {
			return state
		}
	}
	for _, source := range sources {
		if state := nestedStatus(source["video_status"]); state != "" {
			return state
		}
	}
	for _, phase := range []string{"processing_phase", "uploading_phase"} {
		for _, source := range sources {
			if state := nestedStatus(source[phase]); state != "" {
				return state
			}
		}
	}
	for _, source := range sources {
		if state, ok := source["status"].(string); ok {
			return state
		}
	}
	return ""
}

func nestedStatus(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		if state, ok := typed["status"].(string); ok {
			return state
		}
		if state, ok := typed["video_status"].(string); ok {
			return state
		}
	}
	return ""
}

func isFailedState(state string) bool {
	switch strings.ToLower(state) {
	case "error", "failed", "expired":
		return true
	default:
		return false
	}
}

func pageComments(options *globalOptions) *cobra.Command {
	list := operationSpec{Use: "list OBJECT_ID", Short: "List comments on a Page object", Kind: kindRead, Args: cobra.ExactArgs(1), Flags: listFlags, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		all, limit, after, fields := listValues(command)
		if fields == "" {
			fields = "id,message,from,created_time,is_hidden"
		}
		query := url.Values{"fields": {fields}}
		if after != "" {
			query.Set("after", after)
		}
		return client.List(command.Context(), args[0]+"/comments", query, all, limit)
	}}
	var message string
	reply := operationSpec{Use: "reply COMMENT_ID", Short: "Reply to a Page comment", Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&message, "message", "", "reply text")
		_ = command.MarkFlagRequired("message")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		body, _ := json.Marshal(map[string]any{"message": message})
		return client.Write(command.Context(), args[0]+"/comments", nil, body)
	}}
	var hidden bool
	hide := operationSpec{Use: "hide COMMENT_ID", Short: "Hide or unhide a Page comment", Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().BoolVar(&hidden, "hidden", true, "hide when true; unhide when false")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		body, _ := json.Marshal(map[string]any{"is_hidden": hidden})
		return client.Write(command.Context(), args[0], nil, body)
	}}
	remove := operationSpec{Use: "delete COMMENT_ID", Short: "Delete a Page comment", Kind: kindDestructive, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Remove(command.Context(), args[0], nil)
	}}
	return newGroup("comments", "Moderate Page comments", nil, options, list, reply, hide, remove)
}

func pageInsights(options *globalOptions) *cobra.Command {
	var pageMetrics, postMetrics, period string
	page := operationSpec{Use: "page", Short: "Get Page insights", Kind: kindRead, Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&pageMetrics, "metrics", "", "comma-separated metrics supported by the configured Graph version")
		command.Flags().StringVar(&period, "period", "day", "metric period")
		_ = command.MarkFlagRequired("metrics")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		pageID, err := requireID(account.PageID, "page-id")
		if err != nil {
			return nil, err
		}
		return client.Read(command.Context(), pageID+"/insights", url.Values{"metric": {pageMetrics}, "period": {period}})
	}}
	post := operationSpec{Use: "post POST_ID", Short: "Get Page post insights", Kind: kindRead, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&postMetrics, "metrics", "", "comma-separated metrics supported by the configured Graph version")
		_ = command.MarkFlagRequired("metrics")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0]+"/insights", url.Values{"metric": {postMetrics}})
	}}
	return newGroup("insights", "Read Page and post insights", nil, options, page, post)
}
