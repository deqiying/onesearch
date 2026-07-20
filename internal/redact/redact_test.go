package redact

import (
	"reflect"
	"strings"
	"testing"
)

func TestDataMasksSensitiveFieldsEnvironmentValuesAndKnownSecrets(t *testing.T) {
	input := map[string]any{
		"api_key":     "config-secret",
		"api_key_env": "EXA_API_KEY",
		"nested": []map[string]any{{
			"authorization": "Bearer config-secret",
			"message":       "remote echoed env-secret",
		}},
		"settings": map[string]any{
			"env": map[string]string{
				"NORMAL_MODE":  "true",
				"ACCESS_TOKEN": "env-secret",
			},
		},
	}

	safe := Data(input, []string{"env-secret", "config-secret"}).(map[string]any)
	if safe["api_key"] != Mask || safe["api_key_env"] != "EXA_API_KEY" {
		t.Fatalf("credential fields = %#v", safe)
	}
	nested := safe["nested"].([]map[string]any)[0]
	if nested["authorization"] != Mask || strings.Contains(nested["message"].(string), "env-secret") {
		t.Fatalf("nested data was not redacted: %#v", nested)
	}
	env := safe["settings"].(map[string]any)["env"].(map[string]string)
	if env["NORMAL_MODE"] != Mask || env["ACCESS_TOKEN"] != Mask {
		t.Fatalf("settings.env values = %#v", env)
	}
	if input["api_key"] != "config-secret" {
		t.Fatalf("input was mutated: %#v", input)
	}
	if input["settings"].(map[string]any)["env"].(map[string]string)["NORMAL_MODE"] != "true" {
		t.Fatalf("nested input was mutated: %#v", input)
	}
}

func TestCollectSensitiveValuesKeepsOnlyCredentialLikeEnvironmentEntries(t *testing.T) {
	data := map[string]any{
		"api_key":     "direct-secret",
		"api_key_env": "EXA_API_KEY",
		"settings": map[string]any{
			"env": map[string]any{
				"ACCESS_TOKEN": "env-secret",
				"MODE":         "true",
			},
		},
	}
	want := []string{"direct-secret", "env-secret"}
	if got := CollectSensitiveValues(data); !reflect.DeepEqual(got, want) {
		t.Fatalf("sensitive values = %#v, want %#v", got, want)
	}
}

func TestCollectAPIKeyEnvironmentNamesFindsNestedDeclarations(t *testing.T) {
	data := map[string]any{
		"providers": map[string]any{
			"one": map[string]any{"api_key_env": "custom_api_key"},
			"two": map[string]any{"API-KEY-ENV": "CUSTOM_API_KEY"},
		},
	}
	if got := CollectAPIKeyEnvironmentNames(data); !reflect.DeepEqual(got, []string{"CUSTOM_API_KEY"}) {
		t.Fatalf("environment names = %#v", got)
	}
}

func TestTextUsesLiteralLongestFirstReplacement(t *testing.T) {
	rendered := Text("long-secret and secret and a+b", []string{"secret", "long-secret", "a+b", ""})
	if rendered != Mask+" and "+Mask+" and "+Mask {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestIsSensitiveNamePreservesSafeCredentialMetadata(t *testing.T) {
	for _, name := range []string{"api_key_env", "api-key-src", "api_key_set", "api_key_env_set", "has_api_key"} {
		if IsSensitiveName(name) {
			t.Fatalf("safe metadata %q was classified as sensitive", name)
		}
	}
	for _, name := range []string{"api_key", "X-API-Key", "Authorization", "access-token", "client_secret", "PASSWORD"} {
		if !IsSensitiveName(name) {
			t.Fatalf("credential field %q was not classified as sensitive", name)
		}
	}
	for _, name := range []string{"KEY", "AWS_ACCESS_KEY_ID", "SECRET_KEY", "SERVICE_CREDENTIAL_FILE"} {
		if !IsSensitiveEnvironmentName(name) {
			t.Fatalf("credential environment name %q was not classified as sensitive", name)
		}
	}
	if IsSensitiveEnvironmentName("MODE") {
		t.Fatal("generic environment name MODE should not be classified as sensitive")
	}
}
