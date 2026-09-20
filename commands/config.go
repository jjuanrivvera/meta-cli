package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/config"
)

func init() {
	registerMeta(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newConfigCmd(options)) })
}

func newConfigCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Inspect and edit non-secret account settings"}
	command.AddCommand(newConfigPathCmd(options), newConfigViewCmd(options), newConfigSetCmd(options), newConfigUseCmd(options), newConfigListCmd(options), newConfigRemoveCmd(options))
	markLocal(command)
	return command
}

func newConfigPathCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "path", Short: "Print the config file path", RunE: func(command *cobra.Command, _ []string) error {
		configPath := options.deps.ConfigPath
		if configPath == "" {
			var err error
			configPath, err = config.Path()
			if err != nil {
				return err
			}
		}
		fmt.Fprintln(command.OutOrStdout(), configPath)
		return nil
	}}
	markLocal(command)
	return command
}

func newConfigViewCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "view", Short: "Show configuration without credentials", RunE: func(_ *cobra.Command, _ []string) error {
		value, err := config.Load(options.deps.ConfigPath)
		if err != nil {
			return err
		}
		return options.render(value, nil)
	}}
	markLocal(command)
	return command
}

func newConfigSetCmd(options *globalOptions) *cobra.Command {
	var account config.Account
	command := &cobra.Command{Use: "set NAME", Short: "Create or update an account", Args: cobra.ExactArgs(1), Example: "  metactl config set work --graph-version v26.0 --page-id 123 --instagram-id 456", RunE: func(_ *cobra.Command, args []string) error {
		value, err := config.Load(options.deps.ConfigPath)
		if err != nil {
			return err
		}
		existing := value.Accounts[args[0]]
		mergeAccount(&existing, account)
		if err := config.ValidateAccount(args[0], existing); err != nil {
			return err
		}
		value.Accounts[args[0]] = existing
		if value.Current == "" {
			value.Current = args[0]
		}
		return config.Save(options.deps.ConfigPath, value)
	}}
	flags := command.Flags()
	flags.StringVar(&account.BaseURL, "base-url", "", "Graph API base URL")
	flags.StringVar(&account.UploadURL, "upload-url", "", "resumable upload base URL")
	flags.StringVar(&account.GraphVersion, "graph-version", "", "Graph API version")
	flags.StringVar(&account.BusinessID, "business-id", "", "Meta business id")
	flags.StringVar(&account.PageID, "page-id", "", "Facebook Page id")
	flags.StringVar(&account.InstagramID, "instagram-id", "", "Instagram professional account id")
	flags.StringVar(&account.WABAID, "waba-id", "", "WhatsApp Business Account id")
	flags.StringVar(&account.PhoneID, "phone-id", "", "WhatsApp phone number id")
	flags.StringVar(&account.AppID, "app-id", "", "Meta app id")
	flags.Float64Var(&account.RequestsPS, "rps", 0, "request rate ceiling")
	markLocal(command)
	return command
}

func mergeAccount(target *config.Account, source config.Account) {
	if source.BaseURL != "" {
		target.BaseURL = source.BaseURL
	}
	if source.UploadURL != "" {
		target.UploadURL = source.UploadURL
	}
	if source.GraphVersion != "" {
		target.GraphVersion = source.GraphVersion
	}
	if source.BusinessID != "" {
		target.BusinessID = source.BusinessID
	}
	if source.PageID != "" {
		target.PageID = source.PageID
	}
	if source.InstagramID != "" {
		target.InstagramID = source.InstagramID
	}
	if source.WABAID != "" {
		target.WABAID = source.WABAID
	}
	if source.PhoneID != "" {
		target.PhoneID = source.PhoneID
	}
	if source.AppID != "" {
		target.AppID = source.AppID
	}
	if source.RequestsPS != 0 {
		target.RequestsPS = source.RequestsPS
	}
}

func newConfigUseCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "use NAME", Short: "Select the default account", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		value, err := config.Load(options.deps.ConfigPath)
		if err != nil {
			return err
		}
		if _, ok := value.Accounts[args[0]]; !ok {
			return fmt.Errorf("account %q does not exist", args[0])
		}
		value.Current = args[0]
		return config.Save(options.deps.ConfigPath, value)
	}}
	markLocal(command)
	return command
}

func newConfigListCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "list-accounts", Aliases: []string{"list-profiles"}, Short: "List configured accounts", RunE: func(_ *cobra.Command, _ []string) error {
		value, err := config.Load(options.deps.ConfigPath)
		if err != nil {
			return err
		}
		names := make([]string, 0, len(value.Accounts))
		for name := range value.Accounts {
			names = append(names, name)
		}
		sort.Strings(names)
		rows := make([]map[string]any, 0, len(names))
		for _, name := range names {
			rows = append(rows, map[string]any{"name": name, "current": name == value.Current, "graph_version": value.Accounts[name].GraphVersion})
		}
		return options.render(rows, []string{"name", "current", "graph_version"})
	}}
	markLocal(command)
	return command
}

func newConfigRemoveCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "remove NAME", Short: "Remove non-secret account settings", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		value, err := config.Load(options.deps.ConfigPath)
		if err != nil {
			return err
		}
		delete(value.Accounts, args[0])
		if value.Current == args[0] {
			value.Current = ""
		}
		return config.Save(options.deps.ConfigPath, value)
	}}
	markLocal(command)
	return command
}
