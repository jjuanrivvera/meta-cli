package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func promptLine(command *cobra.Command, label string) (string, error) {
	fmt.Fprint(command.ErrOrStderr(), label)
	reader := command.InOrStdin()
	var value strings.Builder
	buffer := make([]byte, 1)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			if buffer[0] == '\n' {
				break
			}
			value.WriteByte(buffer[0])
		}
		if err != nil {
			if err == io.EOF && value.Len() > 0 {
				break
			}
			return "", err
		}
	}
	return strings.TrimSpace(value.String()), nil
}

func promptSecret(command *cobra.Command, label string) (string, error) {
	fmt.Fprint(command.ErrOrStderr(), label)
	if file, ok := command.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		secret, err := readSecretRaw(file)
		fmt.Fprintln(command.ErrOrStderr())
		if err != nil {
			return "", err
		}
		return sanitizeSecret(secret), nil
	}
	return promptLine(command, "")
}

// Raw mode avoids the operating system's canonical line limit for long pasted tokens.
func readSecretRaw(file *os.File) (string, error) {
	fd := int(file.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer func() { _ = term.Restore(fd, oldState) }()
	var value []byte
	buffer := make([]byte, 256)
	for {
		count, readErr := file.Read(buffer)
		for _, character := range buffer[:count] {
			switch character {
			case '\r', '\n':
				return string(value), nil
			case 3:
				return "", fmt.Errorf("cancelled")
			case 8, 127:
				if len(value) > 0 {
					value = value[:len(value)-1]
				}
			default:
				value = append(value, character)
			}
		}
		if readErr != nil {
			if len(value) == 0 {
				return "", readErr
			}
			return string(value), nil
		}
	}
}

func sanitizeSecret(value string) string {
	value = strings.ReplaceAll(value, "\x1b[200~", "")
	value = strings.ReplaceAll(value, "\x1b[201~", "")
	return strings.TrimSpace(value)
}
