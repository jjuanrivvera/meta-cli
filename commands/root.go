package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/api"
	"github.com/jjuanrivvera/meta-cli/internal/auth"
	"github.com/jjuanrivvera/meta-cli/internal/config"
	"github.com/jjuanrivvera/meta-cli/internal/output"
)

const ProfileFlag = "account"

type Dependencies struct {
	In         io.Reader
	Out        io.Writer
	Err        io.Writer
	HTTPClient *http.Client
	ConfigPath string
	Store      auth.Store
}

type globalOptions struct {
	deps         Dependencies
	output       string
	account      string
	baseURL      string
	uploadURL    string
	graphVersion string
	dryRun       bool
	showToken    bool
	verbose      bool
	noColor      bool
	columns      string
	quiet        bool
	jq           string
	sort         string
	filter       string
	pageID       string
	instagramID  string
	businessID   string
	wabaID       string
	phoneID      string
	appID        string
}

type registrar func(*cobra.Command, *globalOptions)

var apiRegistrars []registrar
var metaRegistrars []registrar

func registerAPI(registrar registrar)  { apiRegistrars = append(apiRegistrars, registrar) }
func registerMeta(registrar registrar) { metaRegistrars = append(metaRegistrars, registrar) }

func NewRootCmd(deps Dependencies) *cobra.Command {
	if deps.In == nil {
		deps.In = os.Stdin
	}
	if deps.Out == nil {
		deps.Out = os.Stdout
	}
	if deps.Err == nil {
		deps.Err = os.Stderr
	}
	if deps.Store == nil {
		deps.Store = auth.NewStore(deps.ConfigPath)
	}
	options := &globalOptions{deps: deps, output: output.FormatTable}
	root := &cobra.Command{
		Use:           "metactl",
		Short:         "Publish and manage Meta business content",
		Long:          "metactl manages Instagram publishing, Facebook Pages, and WhatsApp Business through one Graph API client.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Example:       "  metactl pages posts list --page-id 123\n  metactl instagram publish reel --instagram-id 456 --video ./reel.mp4 --dry-run\n  metactl whatsapp send text --phone-id 789 --to 15551234567 --message 'Hello'",
	}
	root.SetIn(deps.In)
	root.SetOut(deps.Out)
	root.SetErr(deps.Err)
	flags := root.PersistentFlags()
	flags.StringVarP(&options.output, "output", "o", output.FormatTable, "output format: table, json, yaml, csv, or id")
	flags.StringVar(&options.account, ProfileFlag, "", "named account to use")
	flags.StringVar(&options.account, "profile", "", "alias for --account")
	_ = flags.MarkHidden("profile")
	flags.StringVar(&options.baseURL, "base-url", "", "Graph API base URL")
	flags.StringVar(&options.uploadURL, "upload-url", "", "resumable upload base URL")
	flags.StringVar(&options.graphVersion, "graph-version", "", "Graph API version")
	flags.BoolVar(&options.dryRun, "dry-run", false, "print equivalent curl requests without sending them")
	flags.BoolVar(&options.showToken, "show-token", false, "show the access token in dry-run output")
	flags.BoolVarP(&options.verbose, "verbose", "v", false, "show request diagnostics")
	flags.BoolVar(&options.noColor, "no-color", false, "disable terminal color")
	flags.StringVar(&options.columns, "columns", "", "comma-separated table or CSV columns")
	flags.BoolVar(&options.quiet, "quiet", false, "suppress non-result messages")
	flags.StringVar(&options.jq, "jq", "", "gojq expression applied before rendering")
	flags.StringVar(&options.sort, "sort", "", "sort result rows by field")
	flags.StringVar(&options.filter, "filter", "", "filter result rows with field=value")
	flags.StringVar(&options.pageID, "page-id", "", "Facebook Page id override")
	flags.StringVar(&options.instagramID, "instagram-id", "", "Instagram professional account id override")
	flags.StringVar(&options.businessID, "business-id", "", "Meta business id override")
	flags.StringVar(&options.wabaID, "waba-id", "", "WhatsApp Business Account id override")
	flags.StringVar(&options.phoneID, "phone-id", "", "WhatsApp phone number id override")
	flags.StringVar(&options.appID, "app-id", "", "Meta app id override")
	for _, registrar := range apiRegistrars {
		registrar(root, options)
	}
	for _, registrar := range metaRegistrars {
		registrar(root, options)
	}
	return root
}

func (options *globalOptions) loadAccount() (string, config.Account, error) {
	value, err := config.Load(options.deps.ConfigPath)
	if err != nil {
		return "", config.Account{}, err
	}
	name := config.FirstNonEmpty(options.account, os.Getenv("METACTL_ACCOUNT"), value.Current, "default")
	account := value.Accounts[name]
	account.BaseURL = config.FirstNonEmpty(options.baseURL, os.Getenv("METACTL_BASE_URL"), account.BaseURL, "https://graph.facebook.com")
	account.UploadURL = config.FirstNonEmpty(options.uploadURL, os.Getenv("METACTL_UPLOAD_URL"), account.UploadURL, "https://rupload.facebook.com")
	account.GraphVersion = config.FirstNonEmpty(options.graphVersion, os.Getenv("METACTL_GRAPH_VERSION"), account.GraphVersion, config.DefaultGraphVersion)
	account.PageID = config.FirstNonEmpty(options.pageID, os.Getenv("METACTL_PAGE_ID"), account.PageID)
	account.InstagramID = config.FirstNonEmpty(options.instagramID, os.Getenv("METACTL_INSTAGRAM_ID"), account.InstagramID)
	account.BusinessID = config.FirstNonEmpty(options.businessID, os.Getenv("METACTL_BUSINESS_ID"), account.BusinessID)
	account.WABAID = config.FirstNonEmpty(options.wabaID, os.Getenv("METACTL_WABA_ID"), account.WABAID)
	account.PhoneID = config.FirstNonEmpty(options.phoneID, os.Getenv("METACTL_PHONE_ID"), account.PhoneID)
	account.AppID = config.FirstNonEmpty(options.appID, os.Getenv("METACTL_APP_ID"), account.AppID)
	return name, account, nil
}

func (options *globalOptions) clientFor() (*api.Client, string, config.Account, error) {
	name, account, err := options.loadAccount()
	if err != nil {
		return nil, "", config.Account{}, err
	}
	credential, storeErr := options.credentialFor(name)
	if storeErr != nil && !options.dryRun {
		return nil, "", config.Account{}, fmt.Errorf("load credential for account %q: %w; run metactl auth login", name, storeErr)
	}
	client, err := api.New(api.Options{
		BaseURL: account.BaseURL, UploadURL: account.UploadURL, Version: account.GraphVersion,
		Token: credential.Token, AppSecret: credential.AppSecret, DryRun: options.dryRun,
		ShowToken: options.showToken, Writer: options.deps.Out, HTTPClient: options.deps.HTTPClient,
		RequestsPS: account.RequestsPS,
	})
	return client, name, account, err
}

func (options *globalOptions) credentialFor(name string) (auth.Credential, error) {
	credential, err := options.deps.Store.Get(name)
	if token := os.Getenv("METACTL_TOKEN"); token != "" {
		credential.Token = token
		err = nil
	}
	if secret := os.Getenv("METACTL_APP_SECRET"); secret != "" {
		credential.AppSecret = secret
	}
	return credential, err
}

func (options *globalOptions) render(value any, preferred []string) error {
	columns := preferred
	if options.columns != "" {
		columns = splitComma(options.columns)
	}
	return output.New(output.Options{
		Format: options.output, Columns: columns, JQ: options.jq, Sort: options.sort,
		Filter: options.filter, NoColor: options.noColor || os.Getenv("NO_COLOR") != "",
		Writer: options.deps.Out, Warnings: options.deps.Err,
	}).Render(value)
}

func splitComma(value string) []string {
	parts := strings.Split(value, ",")
	result := parts[:0]
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
