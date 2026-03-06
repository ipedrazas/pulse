package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestFprintTable_HeadersAndRows(t *testing.T) {
	var buf bytes.Buffer
	fprintTable(&buf, []string{"NAME", "STATUS"}, [][]string{
		{"web", "running"},
		{"db", "exited"},
	})
	out := buf.String()
	if !containsAll(out, "NAME", "STATUS", "web", "running", "db", "exited") {
		t.Errorf("output missing expected content: %s", out)
	}
}

func TestFprintTable_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	fprintTable(&buf, []string{"NAME"}, nil)
	out := buf.String()
	if !containsAll(out, "NAME") {
		t.Errorf("output missing headers: %s", out)
	}
}

func TestFprintJSON_Encoding(t *testing.T) {
	var buf bytes.Buffer
	err := fprintJSON(&buf, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("fprintJSON returned error: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("key = %q, want value", result["key"])
	}
}

func TestFprintJSON_Nil(t *testing.T) {
	var buf bytes.Buffer
	err := fprintJSON(&buf, nil)
	if err != nil {
		t.Fatalf("fprintJSON returned error: %v", err)
	}
	if buf.String() != "null\n" {
		t.Errorf("output = %q, want null\\n", buf.String())
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !bytes.Contains([]byte(s), []byte(sub)) {
			return false
		}
	}
	return true
}
