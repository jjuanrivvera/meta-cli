package commands

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/api"
	"github.com/jjuanrivvera/meta-cli/internal/auth"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

func init() {
	registerMeta(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newAuthCmd(options)) })
}

func newAuthCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "auth", Short: "Manage Graph API credentials"}
	command.AddCommand(
		newAuthLoginCmd(options), newAuthLogoutCmd(options), newAuthStatusCmd(options),
		newAuthDebugCmd(options), newAuthExchangeCmd(options), newAuthPagesCmd(options),
	)
	markLocal(command)
	return command
}

func newAuthLoginCmd(options *globalOptions) *cobra.Command {
	var token string
	var promptAppSecret bool
	command := &cobra.Command{
		Use:   "login",
		Short: "Verify and store an access token",
		Example: "  meta auth login --account work\n" +
			"  printf '%s\\n' \"$TOKEN\" | meta auth login --account ci",
		RunE: func(command *cobra.Command, _ []string) error {
			name, account, err := options.loadAccount()
			if err != nil {
				return err
			}
			if token == "" {
				token, err = promptSecret(command, "Access token: ")
				if err != nil {
					return fmt.Errorf("read token: %w", err)
				}
			}
			if token == "" {
				return fmt.Errorf("access token is required")
			}
			appSecret := os.Getenv("META_APP_SECRET")
			if promptAppSecret && appSecret == "" {
				appSecret, err = promptSecret(command, "App secret: ")
				if err != nil {
					return fmt.Errorf("read app secret: %w", err)
				}
			}
			credential, _ := options.deps.Store.Get(name)
			options.redactions = []string{
				token, credential.Token, credential.PageToken, appSecret, credential.AppSecret,
				os.Getenv("META_TOKEN"), os.Getenv("META_PAGE_TOKEN"), os.Getenv("META_APP_SECRET"),
			}
			client, err := api.New(api.Options{BaseURL: account.BaseURL, UploadURL: account.UploadURL, Version: account.GraphVersion, Token: token, AppSecret: appSecret, DryRun: options.dryRun, Redactions: options.redactions, Writer: options.deps.Err, Diagnostics: options.deps.Err, Verbose: options.verbose, HTTPClient: options.deps.HTTPClient})
			if err != nil {
				return err
			}
			var identity map[string]any
			if err := client.JSON(command.Context(), api.Request{Method: http.MethodGet, Path: "me", Query: url.Values{"fields": {"id,name"}}}, &identity); err != nil {
				return fmt.Errorf("verify token: %w", err)
			}
			if options.dryRun {
				return nil
			}
			credential.Token = token
			if appSecret != "" {
				credential.AppSecret = appSecret
			}
			if err := options.deps.Store.Set(name, credential); err != nil {
				return err
			}
			value, err := config.Load(options.deps.ConfigPath)
			if err != nil {
				return err
			}
			account.AuthMethod = "bearer"
			value.Accounts[name] = account
			value.Current = name
			if err := config.Save(options.deps.ConfigPath, value); err != nil {
				return err
			}
			if !options.quiet {
				fmt.Fprintf(command.ErrOrStderr(), "credential stored for account %q in %s\n", name, options.deps.Store.Backend())
			}
			return options.render(identity, []string{"id", "name"})
		},
	}
	command.Flags().StringVar(&token, "token", "", "access token; omit to read it without echo")
	command.Flags().BoolVar(&promptAppSecret, "prompt-app-secret", false, "prompt without echo for an app secret to store")
	annotate(command, kindWrite)
	return command
}

func newAuthLogoutCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "logout", Short: "Delete the stored credential", RunE: func(command *cobra.Command, _ []string) error {
		name, _, err := options.loadAccount()
		if err != nil {
			return err
		}
		if options.dryRun {
			if !options.quiet {
				fmt.Fprintf(command.ErrOrStderr(), "dry-run: credential for account %q would be removed\n", name)
			}
			return nil
		}
		if err := options.deps.Store.Delete(name); err != nil && !errors.Is(err, auth.ErrNotFound) {
			return err
		}
		if !options.quiet {
			fmt.Fprintf(command.ErrOrStderr(), "credential removed for account %q\n", name)
		}
		return nil
	}}
	annotate(command, kindDestructive)
	return command
}

func newAuthStatusCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "status", Aliases: []string{"whoami"}, Short: "Verify the active identity", Example: "  meta auth status -o json", RunE: func(command *cobra.Command, _ []string) error {
		client, name, account, err := options.clientFor()
		if err != nil {
			return err
		}
		var identity map[string]any
		if err := client.JSON(command.Context(), api.Request{Method: http.MethodGet, Path: "me", Query: url.Values{"fields": {"id,name"}}}, &identity); err != nil {
			return err
		}
		identity["account"] = name
		identity["graph_version"] = account.GraphVersion
		identity["credential_backend"] = options.deps.Store.Backend()
		credential, _ := options.credentialFor(name)
		identity["page_credential_configured"] = credential.PageToken != ""
		return options.render(identity, []string{"account", "id", "name", "graph_version", "credential_backend", "page_credential_configured"})
	}}
	annotate(command, kindRead)
	return command
}

func newAuthDebugCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "debug", Short: "Inspect the active token with debug_token", Example: "  meta auth debug -o json", RunE: func(command *cobra.Command, _ []string) error {
		client, name, account, err := options.clientFor()
		if err != nil {
			return err
		}
		credential, err := options.credentialFor(name)
		if err != nil {
			return err
		}
		query := url.Values{"input_token": {credential.Token}, "access_token": {credential.Token}}
		if account.AppID != "" && credential.AppSecret != "" {
			query.Set("access_token", account.AppID+"|"+credential.AppSecret)
		}
		var result any
		if err := client.JSON(command.Context(), api.Request{Method: http.MethodGet, Path: "debug_token", Query: query, SkipAuthorization: true, SkipAppSecretProof: true}, &result); err != nil {
			return err
		}
		return options.render(result, nil)
	}}
	annotate(command, kindRead)
	return command
}

func newAuthExchangeCmd(options *globalOptions) *cobra.Command {
	var save bool
	command := &cobra.Command{Use: "exchange", Short: "Exchange the active user token for a long-lived token", Example: "  meta auth exchange --save", RunE: func(command *cobra.Command, _ []string) error {
		client, name, account, err := options.clientFor()
		if err != nil {
			return err
		}
		credential, err := options.credentialFor(name)
		if err != nil {
			return err
		}
		if account.AppID == "" || credential.AppSecret == "" {
			return fmt.Errorf("app id and app secret are required; set --app-id and META_APP_SECRET, or run auth login --prompt-app-secret")
		}
		query := url.Values{"grant_type": {"fb_exchange_token"}, "client_id": {account.AppID}, "client_secret": {credential.AppSecret}, "fb_exchange_token": {credential.Token}}
		var result struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			ExpiresIn   int    `json:"expires_in"`
		}
		if err := client.JSON(command.Context(), api.Request{Method: http.MethodGet, Path: "oauth/access_token", Query: query, SkipAuthorization: true, SkipAppSecretProof: true}, &result); err != nil {
			return err
		}
		if save && !options.dryRun {
			credential.Token = result.AccessToken
			if err := options.deps.Store.Set(name, credential); err != nil {
				return err
			}
		}
		return options.render(map[string]any{"token_type": result.TokenType, "expires_in": result.ExpiresIn, "saved": save}, []string{"token_type", "expires_in", "saved"})
	}}
	command.Flags().BoolVar(&save, "save", false, "replace the stored token after a successful exchange")
	annotate(command, kindWrite)
	return command
}

func newAuthPagesCmd(options *globalOptions) *cobra.Command {
	var save bool
	command := &cobra.Command{Use: "pages", Short: "List Pages and derived Page access tokens", Example: "  meta auth pages -o json", RunE: func(command *cobra.Command, _ []string) error {
		client, name, _, err := options.clientFor()
		if err != nil {
			return err
		}
		items, err := client.List(command.Context(), "me/accounts", url.Values{"fields": {"id,name,access_token,tasks"}}, true, 0)
		if err != nil {
			return err
		}
		var result any = items
		for _, item := range items {
			if page, ok := item.(map[string]any); ok {
				if pageToken, ok := page["access_token"].(string); ok && pageToken != "" {
					options.redactions = append(options.redactions, pageToken)
				}
			}
		}
		if save && !options.dryRun {
			if options.pageID == "" {
				return fmt.Errorf("--page-id is required with --save")
			}
			pageToken := findPageToken(result, options.pageID)
			if pageToken == "" {
				return fmt.Errorf("page %q was not returned by /me/accounts", options.pageID)
			}
			credential, getErr := options.credentialFor(name)
			if getErr != nil {
				return getErr
			}
			credential.PageToken = pageToken
			if err := options.deps.Store.Set(name, credential); err != nil {
				return err
			}
			if !options.quiet {
				fmt.Fprintf(command.ErrOrStderr(), "Page access token stored for Page %q\n", options.pageID)
			}
		}
		return options.render(result, []string{"id", "name", "tasks"})
	}}
	command.Flags().BoolVar(&save, "save", false, "store the selected Page access token for Pages operations")
	annotate(command, kindWrite)
	return command
}

func findPageToken(value any, pageID string) string {
	items, ok := value.([]any)
	if !ok {
		object, objectOK := value.(map[string]any)
		if !objectOK {
			return ""
		}
		items, ok = object["data"].([]any)
		if !ok {
			return ""
		}
	}
	for _, item := range items {
		page, ok := item.(map[string]any)
		if ok && stringField(page, "id") == pageID {
			return stringField(page, "access_token")
		}
	}
	return ""
}
