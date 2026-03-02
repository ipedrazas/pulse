package redact

import "testing"

func TestEnvs_RedactsMatchingKeys(t *testing.T) {
	envs := map[string]string{
		"DB_PASSWORD":    "supersecret",
		"API_KEY":        "abc123",
		"LOG_LEVEL":      "debug",
		"AWS_SECRET_KEY": "xyz",
		"HOME":           "/root",
	}
	patterns := []string{"PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIAL"}

	result := Envs(envs, patterns)

	if result["DB_PASSWORD"] != "[REDACTED]" {
		t.Errorf("DB_PASSWORD should be redacted, got %q", result["DB_PASSWORD"])
	}
	if result["API_KEY"] != "[REDACTED]" {
		t.Errorf("API_KEY should be redacted, got %q", result["API_KEY"])
	}
	if result["AWS_SECRET_KEY"] != "[REDACTED]" {
		t.Errorf("AWS_SECRET_KEY should be redacted, got %q", result["AWS_SECRET_KEY"])
	}
	if result["LOG_LEVEL"] != "debug" {
		t.Errorf("LOG_LEVEL should NOT be redacted, got %q", result["LOG_LEVEL"])
	}
	if result["HOME"] != "/root" {
		t.Errorf("HOME should NOT be redacted, got %q", result["HOME"])
	}
}

func TestEnvs_CaseInsensitive(t *testing.T) {
	envs := map[string]string{
		"db_password": "secret",
		"Db_Password": "secret2",
	}
	patterns := []string{"PASSWORD"}

	result := Envs(envs, patterns)

	if result["db_password"] != "[REDACTED]" {
		t.Errorf("db_password (lowercase) should be redacted, got %q", result["db_password"])
	}
	if result["Db_Password"] != "[REDACTED]" {
		t.Errorf("Db_Password (mixed case) should be redacted, got %q", result["Db_Password"])
	}
}

func TestEnvs_EmptyPatterns(t *testing.T) {
	envs := map[string]string{
		"DB_PASSWORD": "secret",
		"LOG_LEVEL":   "info",
	}

	result := Envs(envs, nil)
	if result["DB_PASSWORD"] != "secret" {
		t.Errorf("with no patterns, nothing should be redacted")
	}

	result = Envs(envs, []string{})
	if result["DB_PASSWORD"] != "secret" {
		t.Errorf("with empty patterns, nothing should be redacted")
	}
}

func TestEnvs_EmptyEnvs(t *testing.T) {
	result := Envs(map[string]string{}, []string{"PASSWORD"})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestEnvs_DoesNotMutateOriginal(t *testing.T) {
	envs := map[string]string{
		"DB_PASSWORD": "secret",
	}
	patterns := []string{"PASSWORD"}

	_ = Envs(envs, patterns)

	if envs["DB_PASSWORD"] != "secret" {
		t.Error("original map was mutated")
	}
}
