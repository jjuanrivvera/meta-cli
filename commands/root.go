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
	mcpRoot      string
	redactions   []string
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
	safeErr := &credentialWriter{writer: deps.Err, secrets: func() []string {
		secrets := append([]string(nil), options.redactions...)
		return append(secrets, os.Getenv("META_TOKEN"), os.Getenv("META_PAGE_TOKEN"), os.Getenv("META_APP_SECRET"))
	}}
	options.deps.Err = safeErr
	root := &cobra.Command{
		Use:           "meta",
		Short:         "Publish and manage Meta business content",
		Long:          "meta manages Instagram publishing, Facebook Pages, and WhatsApp Business through one Graph API client.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Example:       "  meta pages posts list --page-id 123\n  meta instagram publish reel --instagram-id 456 --video ./reel.mp4 --dry-run\n  meta whatsapp send text --phone-id 789 --to 15551234567 --message 'Hello'",
		PersistentPreRunE: func(command *cobra.Command, _ []string) error {
			// Control-plane inspection never executes an API operation, so it must not unlock the user's keyring.
			if !skipsCredentialPreflight(command) {
				options.prepareRedactions()
			}
			if err := output.Validate(options.output, options.jq); err != nil {
				return err
			}
			return confineCommandFiles(command, options.mcpRoot)
		},
	}
	root.SetIn(deps.In)
	root.SetOut(deps.Out)
	root.SetErr(safeErr)
	flags := root.PersistentFlags()
	flags.StringVarP(&options.output, "output", "o", output.FormatTable, "output format: table, json, yaml, csv, or id")
	flags.StringVar(&options.account, ProfileFlag, "", "named account to use")
	flags.StringVar(&options.account, "profile", "", "alias for --account")
	_ = flags.MarkHidden("profile")
	flags.StringVar(&options.baseURL, "base-url", "", "Graph API base URL")
	flags.StringVar(&options.uploadURL, "upload-url", "", "resumable upload base URL")
	flags.StringVar(&options.graphVersion, "graph-version", "", "Graph API version")
	flags.BoolVar(&options.dryRun, "dry-run", false, "print equivalent curl requests without sending them")
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
	flags.StringVar(&options.mcpRoot, "mcp-root-internal", "", "internal MCP file root")
	_ = flags.MarkHidden("mcp-root-internal")
	for _, registrar := range apiRegistrars {
		registrar(root, options)
	}
	for _, registrar := range metaRegistrars {
		registrar(root, options)
	}
	return root
}

func skipsCredentialPreflight(command *cobra.Command) bool {
	top := command
	for top.Parent() != nil && top.Parent() != command.Root() {
		top = top.Parent()
	}
	credentialFree := map[string]bool{
		"__surface": true, "agent": true, "alias": true, "completion": true,
		"config": true, "init": true, "mcp": true, "update": true, "version": true,
	}
	return credentialFree[top.Name()]
}

func (options *globalOptions) prepareRedactions() {
	options.redactions = []string{os.Getenv("META_TOKEN"), os.Getenv("META_PAGE_TOKEN"), os.Getenv("META_APP_SECRET")}
	name, account, err := options.loadAccount()
	if err != nil {
		return
	}
	stored, _ := options.deps.Store.Get(name)
	effective, _ := options.credentialFor(name)
	options.addCredentialRedactions(account, stored, effective)
}

func (options *globalOptions) loadAccount() (string, config.Account, error) {
	value, err := config.Load(options.deps.ConfigPath)
	if err != nil {
		return "", config.Account{}, err
	}
	name := config.FirstNonEmpty(options.account, os.Getenv("META_ACCOUNT"), value.Current, "default")
	account := value.Accounts[name]
	account.BaseURL = config.FirstNonEmpty(options.baseURL, os.Getenv("META_BASE_URL"), account.BaseURL, "https://graph.facebook.com")
	account.UploadURL = config.FirstNonEmpty(options.uploadURL, os.Getenv("META_UPLOAD_URL"), account.UploadURL, "https://rupload.facebook.com")
	account.GraphVersion = config.FirstNonEmpty(options.graphVersion, os.Getenv("META_GRAPH_VERSION"), account.GraphVersion, config.DefaultGraphVersion)
	account.PageID = config.FirstNonEmpty(options.pageID, os.Getenv("META_PAGE_ID"), account.PageID)
	account.InstagramID = config.FirstNonEmpty(options.instagramID, os.Getenv("META_INSTAGRAM_ID"), account.InstagramID)
	account.BusinessID = config.FirstNonEmpty(options.businessID, os.Getenv("META_BUSINESS_ID"), account.BusinessID)
	account.WABAID = config.FirstNonEmpty(options.wabaID, os.Getenv("META_WABA_ID"), account.WABAID)
	account.PhoneID = config.FirstNonEmpty(options.phoneID, os.Getenv("META_PHONE_ID"), account.PhoneID)
	account.AppID = config.FirstNonEmpty(options.appID, os.Getenv("META_APP_ID"), account.AppID)
	return name, account, nil
}

func (options *globalOptions) clientFor() (*api.Client, string, config.Account, error) {
	return options.clientForCommand(nil)
}

func (options *globalOptions) clientForCommand(command *cobra.Command) (*api.Client, string, config.Account, error) {
	name, account, err := options.loadAccount()
	if err != nil {
		return nil, "", config.Account{}, err
	}
	credential, storeErr := options.credentialFor(name)
	if storeErr != nil && !options.dryRun {
		return nil, "", config.Account{}, fmt.Errorf("load credential for account %q: %w; run meta auth login", name, storeErr)
	}
	stored, _ := options.deps.Store.Get(name)
	options.redactions = options.redactions[:0]
	options.addCredentialRedactions(account, stored, credential)
	pageOperation := command != nil && strings.HasPrefix(command.CommandPath(), "meta pages ") &&
		!strings.HasPrefix(command.CommandPath(), "meta pages accounts ")
	token := credential.Token
	if pageOperation {
		token = credential.PageToken
		if token == "" {
			if !options.dryRun {
				return nil, "", config.Account{}, fmt.Errorf("no Page access token stored for account %q; run meta auth pages --save --page-id PAGE_ID", name)
			}
			token = "<page-access-token>"
		}
	}
	client, err := api.New(api.Options{
		BaseURL: account.BaseURL, UploadURL: account.UploadURL, Version: account.GraphVersion,
		Token: token, AppSecret: credential.AppSecret, DryRun: options.dryRun,
		Redactions: options.redactions,
		Verbose:    options.verbose, Writer: options.deps.Err, Diagnostics: options.deps.Err,
		HTTPClient: options.deps.HTTPClient,
		RequestsPS: account.RequestsPS,
	})
	return client, name, account, err
}

func (options *globalOptions) credentialFor(name string) (auth.Credential, error) {
	credential, err := options.deps.Store.Get(name)
	if token := os.Getenv("META_TOKEN"); token != "" {
		credential.Token = token
		err = nil
	}
	if token := os.Getenv("META_PAGE_TOKEN"); token != "" {
		credential.PageToken = token
		err = nil
	}
	if secret := os.Getenv("META_APP_SECRET"); secret != "" {
		credential.AppSecret = secret
	}
	return credential, err
}

func (options *globalOptions) addCredentialRedactions(account config.Account, credentials ...auth.Credential) {
	for _, credential := range credentials {
		options.redactions = append(options.redactions, credential.Token, credential.PageToken, credential.AppSecret)
		if account.AppID != "" && credential.AppSecret != "" {
			options.redactions = append(options.redactions, account.AppID+"|"+credential.AppSecret)
		}
	}
}

func (options *globalOptions) render(value any, preferred []string) error {
	columns := preferred
	if options.columns != "" {
		columns = splitComma(options.columns)
	}
	return output.New(output.Options{
		Format: options.output, Columns: columns, JQ: options.jq, Sort: options.sort,
		Filter: options.filter, NoColor: options.noColor || os.Getenv("NO_COLOR") != "",
		Writer: options.deps.Out, Warnings: options.deps.Err, Secrets: options.redactions,
	}).Render(value)
}

func (options *globalOptions) renderPartialFallback(value any) error {
	return output.New(output.Options{
		Format: output.FormatJSON, Writer: options.deps.Out, Warnings: options.deps.Err,
		NoColor: true, Secrets: options.redactions,
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
