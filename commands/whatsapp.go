package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/api"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

func init() {
	registerAPI(func(root *cobra.Command, options *globalOptions) {
		whatsapp := &cobra.Command{Use: "whatsapp", Aliases: []string{"wa"}, Short: "Manage WhatsApp Business Cloud API resources"}
		whatsapp.AddCommand(
			whatsappAccounts(options), whatsappPhones(options), whatsappTemplates(options),
			whatsappApps(options), whatsappProfile(options), whatsappMedia(options), whatsappSend(options),
		)
		root.AddCommand(whatsapp)
	})
}

func whatsappAccounts(options *globalOptions) *cobra.Command {
	list := operationSpec{Use: "list", Short: "List WhatsApp Business Accounts owned by a business", Kind: kindRead, Flags: listFlags, Columns: []string{"id", "name", "currency", "timezone_id"}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		businessID, err := requireID(account.BusinessID, "business-id")
		if err != nil {
			return nil, err
		}
		all, limit, after, fields := listValues(command)
		if fields == "" {
			fields = "id,name,currency,timezone_id,message_template_namespace"
		}
		query := url.Values{"fields": {fields}}
		if after != "" {
			query.Set("after", after)
		}
		return client.List(command.Context(), businessID+"/owned_whatsapp_business_accounts", query, all, limit)
	}}
	get := operationSpec{Use: "get", Short: "Get the configured WhatsApp Business Account", Kind: kindRead, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		wabaID, err := requireID(account.WABAID, "waba-id")
		if err != nil {
			return nil, err
		}
		return client.Read(command.Context(), wabaID, url.Values{"fields": {"id,name,currency,timezone_id,message_template_namespace"}})
	}}
	return newGroup("accounts", "Manage WhatsApp Business Accounts", nil, options, list, get)
}

func whatsappPhones(options *globalOptions) *cobra.Command {
	list := operationSpec{Use: "list", Short: "List phone numbers and operational status", Kind: kindRead, Flags: listFlags, Columns: []string{"id", "display_phone_number", "verified_name", "quality_rating", "name_status", "code_verification_status", "platform_type"}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		wabaID, err := requireID(account.WABAID, "waba-id")
		if err != nil {
			return nil, err
		}
		all, limit, after, fields := listValues(command)
		if fields == "" {
			fields = "id,display_phone_number,verified_name,quality_rating,name_status,code_verification_status,platform_type,throughput"
		}
		query := url.Values{"fields": {fields}}
		if after != "" {
			query.Set("after", after)
		}
		return client.List(command.Context(), wabaID+"/phone_numbers", query, all, limit)
	}}
	get := operationSpec{Use: "get [PHONE_ID]", Short: "Get phone status, quality, display-name, and registration fields", Kind: kindRead, Args: cobra.MaximumNArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, args []string) (any, error) {
		id := account.PhoneID
		if len(args) == 1 {
			id = args[0]
		}
		id, err := requireID(id, "phone-id")
		if err != nil {
			return nil, err
		}
		return client.Read(command.Context(), id, url.Values{"fields": {"id,display_phone_number,verified_name,quality_rating,name_status,code_verification_status,platform_type,throughput"}})
	}}
	return newGroup("phones", "Inspect WhatsApp phone numbers", []string{"phone"}, options, list, get)
}

func whatsappTemplates(options *globalOptions) *cobra.Command {
	list := operationSpec{Use: "list", Short: "List message templates", Kind: kindRead, Flags: listFlags, Columns: []string{"id", "name", "language", "category", "status", "quality_score"}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		wabaID, err := requireID(account.WABAID, "waba-id")
		if err != nil {
			return nil, err
		}
		all, limit, after, fields := listValues(command)
		if fields == "" {
			fields = "id,name,language,category,status,quality_score,components"
		}
		query := url.Values{"fields": {fields}}
		if after != "" {
			query.Set("after", after)
		}
		return client.List(command.Context(), wabaID+"/message_templates", query, all, limit)
	}}
	get := operationSpec{Use: "get TEMPLATE_ID", Short: "Get a message template", Kind: kindRead, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0], url.Values{"fields": {"id,name,language,category,status,quality_score,components"}})
	}}
	var createWrite writeOptions
	var name, language, category, components string
	create := operationSpec{Use: "create", Short: "Create a message template", Kind: kindWrite, Example: "  metactl whatsapp templates create --name order_ready --language en_US --category UTILITY --components '[{\"type\":\"BODY\",\"text\":\"Order {{1}} is ready\"}]'", Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&name, "name", "", "template name")
		command.Flags().StringVar(&language, "language", "en_US", "template language")
		command.Flags().StringVar(&category, "category", "UTILITY", "template category")
		command.Flags().StringVar(&components, "components", "", "JSON component array")
		createWrite.addFlags(command)
		_ = command.MarkFlagRequired("name")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		wabaID, err := requireID(account.WABAID, "waba-id")
		if err != nil {
			return nil, err
		}
		defaults := map[string]any{"name": name, "language": language, "category": category}
		if components != "" {
			var parsed any
			if err := json.Unmarshal([]byte(components), &parsed); err != nil {
				return nil, fmt.Errorf("decode --components: %w", err)
			}
			defaults["components"] = parsed
		}
		body, err := createWrite.body(command, defaults)
		if err != nil {
			return nil, err
		}
		return client.Write(command.Context(), wabaID+"/message_templates", nil, body)
	}}
	var updateWrite writeOptions
	var updateCategory, updateComponents string
	update := operationSpec{Use: "update TEMPLATE_ID", Short: "Update a message template", Kind: kindWrite, Args: cobra.ExactArgs(1), Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&updateCategory, "category", "", "new category")
		command.Flags().StringVar(&updateComponents, "components", "", "JSON component array")
		updateWrite.addFlags(command)
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		defaults := map[string]any{"category": updateCategory}
		if updateComponents != "" {
			var parsed any
			if err := json.Unmarshal([]byte(updateComponents), &parsed); err != nil {
				return nil, err
			}
			defaults["components"] = parsed
		}
		body, err := updateWrite.body(command, defaults)
		if err != nil {
			return nil, err
		}
		return client.Write(command.Context(), args[0], nil, body)
	}}
	var deleteName string
	remove := operationSpec{Use: "delete", Short: "Delete a message template by name", Kind: kindDestructive, Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&deleteName, "name", "", "template name")
		_ = command.MarkFlagRequired("name")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		wabaID, err := requireID(account.WABAID, "waba-id")
		if err != nil {
			return nil, err
		}
		return client.Remove(command.Context(), wabaID+"/message_templates", url.Values{"name": {deleteName}})
	}}
	return newGroup("templates", "Manage WhatsApp message templates", []string{"template"}, options, list, get, create, update, remove)
}

func whatsappApps(options *globalOptions) *cobra.Command {
	list := operationSpec{Use: "list", Short: "List subscribed apps", Kind: kindRead, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		wabaID, err := requireID(account.WABAID, "waba-id")
		if err != nil {
			return nil, err
		}
		return client.Read(command.Context(), wabaID+"/subscribed_apps", nil)
	}}
	subscribe := operationSpec{Use: "subscribe", Short: "Subscribe the current app", Kind: kindWrite, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		wabaID, err := requireID(account.WABAID, "waba-id")
		if err != nil {
			return nil, err
		}
		return client.Write(command.Context(), wabaID+"/subscribed_apps", nil, []byte(`{}`))
	}}
	unsubscribe := operationSpec{Use: "unsubscribe", Short: "Unsubscribe the current app", Kind: kindDestructive, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		wabaID, err := requireID(account.WABAID, "waba-id")
		if err != nil {
			return nil, err
		}
		return client.Remove(command.Context(), wabaID+"/subscribed_apps", nil)
	}}
	return newGroup("apps", "Manage WhatsApp app subscriptions", nil, options, list, subscribe, unsubscribe)
}

func whatsappProfile(options *globalOptions) *cobra.Command {
	get := operationSpec{Use: "get", Short: "Get the WhatsApp business profile", Kind: kindRead, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		phoneID, err := requireID(account.PhoneID, "phone-id")
		if err != nil {
			return nil, err
		}
		return client.Read(command.Context(), phoneID+"/whatsapp_business_profile", url.Values{"fields": {"about,address,description,email,profile_picture_url,websites,vertical"}})
	}}
	var write writeOptions
	var about, address, description, email, vertical string
	var websites []string
	update := operationSpec{Use: "update", Short: "Update the WhatsApp business profile", Kind: kindWrite, Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&about, "about", "", "about text")
		command.Flags().StringVar(&address, "address", "", "business address")
		command.Flags().StringVar(&description, "description", "", "business description")
		command.Flags().StringVar(&email, "email", "", "business email")
		command.Flags().StringSliceVar(&websites, "websites", nil, "website URLs")
		command.Flags().StringVar(&vertical, "vertical", "", "business vertical")
		write.addFlags(command)
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		phoneID, err := requireID(account.PhoneID, "phone-id")
		if err != nil {
			return nil, err
		}
		body, err := write.body(command, map[string]any{"messaging_product": "whatsapp", "about": about, "address": address, "description": description, "email": email, "websites": websites, "vertical": vertical})
		if err != nil {
			return nil, err
		}
		return client.Write(command.Context(), phoneID+"/whatsapp_business_profile", nil, body)
	}}
	return newGroup("profile", "Manage the WhatsApp business profile", nil, options, get, update)
}

func whatsappMedia(options *globalOptions) *cobra.Command {
	var filePath, contentType string
	upload := operationSpec{Use: "upload", Short: "Upload WhatsApp media", Kind: kindWrite, Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&filePath, "file", "", "local media file")
		command.Flags().StringVar(&contentType, "content-type", "application/octet-stream", "media MIME type")
		_ = command.MarkFlagRequired("file")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		phoneID, err := requireID(account.PhoneID, "phone-id")
		if err != nil {
			return nil, err
		}
		return uploadWhatsAppMedia(command, client, phoneID, filePath, contentType)
	}}
	get := operationSpec{Use: "get MEDIA_ID", Short: "Get WhatsApp media metadata and download URL", Kind: kindRead, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Read(command.Context(), args[0], nil)
	}}
	remove := operationSpec{Use: "delete MEDIA_ID", Short: "Delete WhatsApp media", Kind: kindDestructive, Args: cobra.ExactArgs(1), Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, _ config.Account, args []string) (any, error) {
		return client.Remove(command.Context(), args[0], nil)
	}}
	return newGroup("media", "Manage WhatsApp media", nil, options, upload, get, remove)
}

func uploadWhatsAppMedia(command *cobra.Command, client *api.Client, phoneID, filePath, contentType string) (any, error) {
	data, err := os.ReadFile(filePath) // #nosec G304 -- the user explicitly selected this upload file
	if err != nil {
		return nil, fmt.Errorf("read media: %w", err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("messaging_product", "whatsapp"); err != nil {
		return nil, err
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(filePath)))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return uploadResponse(command, client, api.Request{Method: http.MethodPost, Path: phoneID + "/media", Body: body.Bytes(), Headers: http.Header{"Content-Type": {writer.FormDataContentType()}}})
}

func whatsappSend(options *globalOptions) *cobra.Command {
	var to, message, templateName, language, components string
	text := operationSpec{Use: "text", Short: "Send a WhatsApp text message (remote side effect)", Kind: kindWrite, Example: "  metactl whatsapp send text --to 15551234567 --message 'Hello'", Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&to, "to", "", "recipient phone number")
		command.Flags().StringVar(&message, "message", "", "message body")
		_ = command.MarkFlagRequired("to")
		_ = command.MarkFlagRequired("message")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		phoneID, err := requireID(account.PhoneID, "phone-id")
		if err != nil {
			return nil, err
		}
		body, _ := json.Marshal(map[string]any{"messaging_product": "whatsapp", "to": to, "type": "text", "text": map[string]any{"body": message}})
		return client.Write(command.Context(), phoneID+"/messages", nil, body)
	}}
	template := operationSpec{Use: "template", Short: "Send a WhatsApp template message (remote side effect)", Kind: kindWrite, Example: "  metactl whatsapp send template --to 15551234567 --name order_ready --language en_US", Flags: func(command *cobra.Command) {
		command.Flags().StringVar(&to, "to", "", "recipient phone number")
		command.Flags().StringVar(&templateName, "name", "", "approved template name")
		command.Flags().StringVar(&language, "language", "en_US", "template language code")
		command.Flags().StringVar(&components, "components", "", "JSON components array")
		_ = command.MarkFlagRequired("to")
		_ = command.MarkFlagRequired("name")
	}, Run: func(command *cobra.Command, _ *globalOptions, client *api.Client, account config.Account, _ []string) (any, error) {
		phoneID, err := requireID(account.PhoneID, "phone-id")
		if err != nil {
			return nil, err
		}
		templateBody := map[string]any{"name": templateName, "language": map[string]any{"code": language}}
		if components != "" {
			var parsed any
			if err := json.Unmarshal([]byte(components), &parsed); err != nil {
				return nil, err
			}
			templateBody["components"] = parsed
		}
		body, _ := json.Marshal(map[string]any{"messaging_product": "whatsapp", "to": to, "type": "template", "template": templateBody})
		return client.Write(command.Context(), phoneID+"/messages", nil, body)
	}}
	return newGroup("send", "Send WhatsApp messages", nil, options, text, template)
}
