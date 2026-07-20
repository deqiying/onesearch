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

func runConfig(svc *service.Service, args []string) int {
	if len(args) == 0 {
		printConfigHelp()
		return 2
	}
	if isHelpToken(args[0]) {
		printConfigHelp()
		return 0
	}
	subcommand := canonicalConfig(args[0])
	switch subcommand {
	case "path", "list":
		return runConfigRead(svc, subcommand, args[1:])
	case "setup":
		return runConfigSetup(svc, args[1:])
	default:
		return parameterError(svc, "unknown config subcommand: "+args[0])
	}
}

func runConfigRead(svc *service.Service, subcommand string, args []string) int {
	if hasHelpToken(args) {
		printConfigHelp()
		return 0
	}
	fs := flagSet("config " + subcommand)
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError(svc, "config", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() > 0 {
		return printParameterError(svc, "config", "config "+subcommand+" does not accept positional arguments", makeFormatOutput(outputFlags, svc))
	}
	if subcommand == "path" {
		return printCommand(svc, "config", svc.ConfigPath(), makeFormatOutput(outputFlags, svc))
	}
	return printCommand(svc, "config", svc.ConfigList(false), makeFormatOutput(outputFlags, svc))
}

func runConfigSetup(svc *service.Service, args []string) int {
	if hasHelpToken(args) {
		printConfigHelp()
		return 0
	}
	for _, arg := range args {
		if arg == "--api-key" || strings.HasPrefix(arg, "--api-key=") {
			return printParameterError(svc, "config", "--api-key is not supported; use hidden input or --api-key-stdin", parseFormatOutput(args, svc))
		}
	}
	fs := flagSet("config setup")
	baseURL := optionalStringFlag{}
	fs.Var(&baseURL, "base-url", "")
	apiKeyStdin := fs.Bool("api-key-stdin", false, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError(svc, "config", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	fo := makeFormatOutput(outputFlags, svc)
	if fs.NArg() != 1 {
		return printParameterError(svc, "config", "config setup requires exactly one provider", fo)
	}
	requestedProvider := fs.Arg(0)
	spec, err := svc.ProviderSetupSpec(requestedProvider)
	if err != nil {
		return printCommand(svc, "config", setupErrorData(svc, requestedProvider, err), fo)
	}

	interactive := configInputIsTerminal() && !*apiKeyStdin
	var apiKey *string
	var transientSecret string
	if *apiKeyStdin {
		value, readErr := readConfigLine(configStdin())
		transientSecret = value
		if readErr != nil {
			return printCommand(svc, "config", configInputErrorData(svc, spec.Provider, readErr), fo, transientSecret)
		}
		apiKey = &value
	} else if interactive {
		prompt := "API key"
		if spec.HasEffectiveAPIKey {
			prompt += " [已配置，留空保留]"
		}
		writeConfigPrompt(svc, prompt+": ")
		value, readErr := configReadPassword()
		fmt.Fprintln(os.Stderr)
		text := strings.TrimSpace(string(value))
		transientSecret = text
		if readErr != nil {
			return printCommand(svc, "config", configInputErrorData(svc, spec.Provider, readErr), fo, transientSecret)
		}
		apiKey = &text
	} else if spec.RequiresAPIKey && !spec.HasEffectiveAPIKey {
		return printParameterError(svc, "config", "non-interactive setup requires --api-key-stdin when no effective API key exists", fo)
	}

	var requestedBaseURL *string
	if baseURL.set {
		value := baseURL.value
		requestedBaseURL = &value
	} else if interactive && spec.SupportsBaseURL {
		prompt := "Base URL"
		if value := strings.TrimSpace(spec.EffectiveBaseURL); value != "" {
			prompt += " [" + safeBaseURLPrompt(value) + "]"
		}
		writeConfigPrompt(svc, redact.Text(prompt+": ", []string{transientSecret}))
		value, readErr := readConfigLine(configStdin())
		if readErr != nil {
			return printCommand(svc, "config", configInputErrorData(svc, spec.Provider, readErr), fo, transientSecret)
		}
		requestedBaseURL = &value
	}

	preSaveSecrets := svc.OutputSecretValues()
	data := svc.SetupProvider(service.ProviderSetupRequest{
		Provider: spec.Provider,
		APIKey:   apiKey,
		BaseURL:  requestedBaseURL,
	})
	return printCommand(svc, "config", data, fo, append(preSaveSecrets, transientSecret)...)
}

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
	return map[string]any{
		"ok":          false,
		"error_type":  errorType,
		"error":       err.Error(),
		"provider":    provider,
		"config_file": svc.Config.ConfigFile,
	}
}

func configInputErrorData(svc *service.Service, provider string, err error) map[string]any {
	return map[string]any{
		"ok":          false,
		"error_type":  "local_error",
		"error":       "无法读取配置输入: " + err.Error(),
		"provider":    provider,
		"config_file": svc.Config.ConfigFile,
	}
}

func printConfigHelp() {
	fmt.Println("onesearch config path [--format json|markdown|content]")
	fmt.Println("onesearch config list [--format json|markdown|content]")
	fmt.Println("onesearch config setup <provider> [--base-url URL] [--api-key-stdin] [--format json|markdown|content]")
}

type optionalStringFlag struct {
	value string
	set   bool
}

func (f *optionalStringFlag) String() string { return f.value }
func (f *optionalStringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}
