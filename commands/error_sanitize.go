package commands

import (
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/jjuanrivvera/meta-cli/internal/auth"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

const minimumCredentialRedactionLength = 8

type credentialWriter struct {
	writer  io.Writer
	secrets func() []string
}

func (writer *credentialWriter) Write(data []byte) (int, error) {
	redacted := redactCommandText(string(data), writer.secrets())
	_, err := io.WriteString(writer.writer, redacted)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

type sanitizedCommandError struct {
	cause   error
	message string
}

func (err *sanitizedCommandError) Error() string { return err.message }
func (err *sanitizedCommandError) Unwrap() error { return err.cause }

func redactCommandError(err error, secrets []string) error {
	if err == nil {
		return nil
	}
	message := redactCommandText(err.Error(), secrets)
	if message == err.Error() {
		return err
	}
	return &sanitizedCommandError{cause: err, message: message}
}

func redactCommandText(message string, secrets []string) string {
	for _, secret := range secrets {
		if len(secret) < minimumCredentialRedactionLength {
			continue
		}
		message = strings.ReplaceAll(message, secret, "<redacted>")
		message = strings.ReplaceAll(message, url.QueryEscape(secret), url.QueryEscape("<redacted>"))
	}
	return message
}

func (options *globalOptions) sanitizeError(err error) error {
	secrets := append([]string(nil), options.redactions...)
	secrets = append(secrets, os.Getenv("META_TOKEN"), os.Getenv("META_PAGE_TOKEN"), os.Getenv("META_APP_SECRET"))
	return redactCommandError(err, secrets)
}

// SanitizeError is the last output-boundary guard for parse and dispatch errors
// that occur before a command can initialize its normal credential redactions.
func SanitizeError(err error, args []string, dependencies Dependencies) error {
	if err == nil {
		return nil
	}
	return redactCommandError(err, commandCredentialSecrets(args, dependencies))
}

func commandCredentialSecrets(args []string, dependencies Dependencies) []string {
	secrets := []string{os.Getenv("META_TOKEN"), os.Getenv("META_PAGE_TOKEN"), os.Getenv("META_APP_SECRET")}
	// Control-plane commands are credential-free and must not prompt to unlock an unrelated host keyring.
	if len(args) > 0 && (args[0] == "__surface" || args[0] == "mcp") {
		return secrets
	}
	accountName := os.Getenv("META_ACCOUNT")
	for index, argument := range args {
		switch {
		case (argument == "--account" || argument == "--profile") && index+1 < len(args):
			accountName = args[index+1]
		case strings.HasPrefix(argument, "--account="):
			accountName = strings.TrimPrefix(argument, "--account=")
		case strings.HasPrefix(argument, "--profile="):
			accountName = strings.TrimPrefix(argument, "--profile=")
		}
	}
	value, loadErr := config.Load(dependencies.ConfigPath)
	if loadErr == nil {
		accountName = config.FirstNonEmpty(accountName, value.Current, "default")
		store := dependencies.Store
		if store == nil {
			store = auth.NewStore(dependencies.ConfigPath)
		}
		if credential, getErr := store.Get(accountName); getErr == nil {
			secrets = append(secrets, credential.Token, credential.PageToken, credential.AppSecret)
			if account := value.Accounts[accountName]; account.AppID != "" && credential.AppSecret != "" {
				secrets = append(secrets, account.AppID+"|"+credential.AppSecret)
			}
		}
	}
	return secrets
}
