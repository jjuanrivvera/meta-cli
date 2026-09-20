package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/auth"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

func init() {
	registerMeta(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newInitCmd(options)) })
}

func newInitCmd(options *globalOptions) *cobra.Command {
	var account config.Account
	var token, appSecret string
	command := &cobra.Command{
		Use:     "init",
		Aliases: []string{"setup"},
		Short:   "Configure an account and store its credential",
		Example: "  metactl init --account work --page-id 123 --instagram-id 456 --waba-id 789 --phone-id 101",
		RunE: func(command *cobra.Command, _ []string) error {
			name := options.account
			if name == "" {
				name = "default"
			}
			if account.BaseURL == "" {
				account.BaseURL = "https://graph.facebook.com"
			}
			if account.UploadURL == "" {
				account.UploadURL = "https://rupload.facebook.com"
			}
			if account.GraphVersion == "" {
				account.GraphVersion = config.DefaultGraphVersion
			}
			account.AuthMethod = "bearer"
			if err := config.ValidateAccount(name, account); err != nil {
				return err
			}
			if token == "" {
				var err error
				token, err = promptSecret(command, "Access token: ")
				if err != nil {
					return err
				}
			}
			if token == "" {
				return fmt.Errorf("access token is required")
			}
			value, err := config.Load(options.deps.ConfigPath)
			if err != nil {
				return err
			}
			value.Accounts[name] = account
			value.Current = name
			if err := config.Save(options.deps.ConfigPath, value); err != nil {
				return err
			}
			if err := options.deps.Store.Set(name, auth.Credential{Token: token, AppSecret: appSecret}); err != nil {
				return err
			}
			if !options.quiet {
				fmt.Fprintf(command.ErrOrStderr(), "account %q configured; run metactl doctor\n", name)
			}
			return nil
		},
	}
	flags := command.Flags()
	flags.StringVar(&token, "token", "", "access token; omit to read it without echo")
	flags.StringVar(&appSecret, "app-secret", "", "optional app secret for appsecret_proof")
	flags.StringVar(&account.BaseURL, "graph-url", "", "Graph API base URL")
	flags.StringVar(&account.UploadURL, "resumable-url", "", "resumable upload base URL")
	flags.StringVar(&account.GraphVersion, "version", "", "Graph API version")
	flags.StringVar(&account.BusinessID, "business", "", "Meta business id")
	flags.StringVar(&account.PageID, "page", "", "Facebook Page id")
	flags.StringVar(&account.InstagramID, "instagram", "", "Instagram professional account id")
	flags.StringVar(&account.WABAID, "waba", "", "WhatsApp Business Account id")
	flags.StringVar(&account.PhoneID, "phone", "", "WhatsApp phone number id")
	flags.StringVar(&account.AppID, "app", "", "Meta app id")
	markLocal(command)
	return command
}
