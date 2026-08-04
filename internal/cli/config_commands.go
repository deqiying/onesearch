package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/deqiying/onesearch/internal/redact"
	"github.com/deqiying/onesearch/internal/service"
	"golang.org/x/term"
)

var (
	configInputIsTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	configReadPassword    = func() ([]byte, error) { return term.ReadPassword(int(os.Stdin.Fd())) }
	configStdin           = func() io.Reader { return os.Stdin }
)

func readConfigLine(reader io.Reader) (string, error) {
	buffered := bufio.NewReader(io.LimitReader(reader, 64*1024))
	value, err := buffered.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return strings.TrimSpace(value), err
	}
	return strings.TrimSpace(value), nil
}

func safeBaseURLPrompt(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || strings.Contains(value, "#") {
		return "configured value omitted"
	}
	return value
}

func writeConfigPrompt(svc *service.Service, value string) {
	secrets := []string{}
	if svc != nil {
		secrets = svc.OutputSecretValues()
	}
	fmt.Fprint(os.Stderr, redact.Text(value, secrets))
}

func setupErrorData(svc *service.Service, provider string, err error) map[string]any {
	errorType := "config_error"
	if typed, ok := err.(*service.ProviderSetupError); ok {
		errorType = typed.ErrorType
	}
	return map[string]any{"ok": false, "error_type": errorType, "error": err.Error(), "provider": provider, "config_file": svc.Config.ConfigFile}
}

func configInputErrorData(svc *service.Service, provider string, err error) map[string]any {
	return map[string]any{"ok": false, "error_type": "local_error", "error": "无法读取配置输入: " + err.Error(), "provider": provider, "config_file": svc.Config.ConfigFile}
}
