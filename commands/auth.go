package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

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
	var token, appSecret string
	command := &cobra.Command{
		Use:   "login",
		Short: "Verify and store an access token",
		Example: "  metactl auth login --account work\n" +
			"  printf '%s\\n' \"$TOKEN\" | metactl auth login --account ci",
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
			client, err := api.New(api.Options{BaseURL: account.BaseURL, UploadURL: account.UploadURL, Version: account.GraphVersion, Token: token, AppSecret: appSecret, DryRun: options.dryRun, ShowToken: options.showToken, Writer: options.deps.Out, HTTPClient: options.deps.HTTPClient})
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
			if err := options.deps.Store.Set(name, auth.Credential{Token: token, AppSecret: appSecret}); err != nil {
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
	command.Flags().StringVar(&appSecret, "app-secret", "", "app secret for appsecret_proof; stored separately")
	annotate(command, kindWrite)
	return command
}

func newAuthLogoutCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "logout", Short: "Delete the stored credential", RunE: func(command *cobra.Command, _ []string) error {
		name, _, err := options.loadAccount()
		if err != nil {
			return err
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
	command := &cobra.Command{Use: "status", Aliases: []string{"whoami"}, Short: "Verify the active identity", Example: "  metactl auth status -o json", RunE: func(command *cobra.Command, _ []string) error {
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
		return options.render(identity, []string{"account", "id", "name", "graph_version", "credential_backend"})
	}}
	annotate(command, kindRead)
	return command
}

func newAuthDebugCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "debug", Short: "Inspect the active token with debug_token", Example: "  metactl auth debug -o json", RunE: func(command *cobra.Command, _ []string) error {
		client, name, account, err := options.clientFor()
		if err != nil {
			return err
		}
		credential, err := options.credentialFor(name)
		if err != nil {
			return err
		}
		query := url.Values{"input_token": {credential.Token}}
		if account.AppID != "" && credential.AppSecret != "" {
			query.Set("access_token", account.AppID+"|"+credential.AppSecret)
		}
		var result any
		if err := client.JSON(command.Context(), api.Request{Method: http.MethodGet, Path: "debug_token", Query: query}, &result); err != nil {
			return err
		}
		return options.render(result, nil)
	}}
	annotate(command, kindRead)
	return command
}

func newAuthExchangeCmd(options *globalOptions) *cobra.Command {
	var save bool
	command := &cobra.Command{Use: "exchange", Short: "Exchange the active user token for a long-lived token", Example: "  metactl auth exchange --save", RunE: func(command *cobra.Command, _ []string) error {
		client, name, account, err := options.clientFor()
		if err != nil {
			return err
		}
		credential, err := options.credentialFor(name)
		if err != nil {
			return err
		}
		if account.AppID == "" || credential.AppSecret == "" {
			return fmt.Errorf("app id and app secret are required; set --app-id and run auth login with --app-secret")
		}
		query := url.Values{"grant_type": {"fb_exchange_token"}, "client_id": {account.AppID}, "client_secret": {credential.AppSecret}, "fb_exchange_token": {credential.Token}}
		var result struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			ExpiresIn   int    `json:"expires_in"`
		}
		if err := client.JSON(command.Context(), api.Request{Method: http.MethodGet, Path: "oauth/access_token", Query: query}, &result); err != nil {
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
	command := &cobra.Command{Use: "pages", Short: "List Pages and derived Page access tokens", Example: "  metactl auth pages -o json", RunE: func(command *cobra.Command, _ []string) error {
		client, _, _, err := options.clientFor()
		if err != nil {
			return err
		}
		response, err := client.Do(command.Context(), api.Request{Method: http.MethodGet, Path: "me/accounts", Query: url.Values{"fields": {"id,name,access_token,tasks"}}})
		if err != nil {
			return err
		}
		var result any
		if err := json.Unmarshal(response.Body, &result); err != nil {
			return err
		}
		return options.render(result, []string{"id", "name", "tasks"})
	}}
	annotate(command, kindRead)
	return command
}
