package redact

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/join-com/pii-redact/internal/config"
	"github.com/join-com/pii-redact/internal/patterns"
)

func buildOpts(t *testing.T, safe, redact []string, strict bool) Options {
	t.Helper()
	set, err := patterns.Compile(map[string]string{
		"email": `[\w.+-]+@[\w-]+\.[\w.-]+`,
		"ipv4":  `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
	})
	if err != nil {
		t.Fatal(err)
	}
	rules := &config.Ruleset{
		SafeKeys:            toSet(safe),
		RedactKeys:          toSet(redact),
		MaxSafeStringLength: 0,
		ServiceOverrides:    map[string]config.ServiceRules{},
	}
	return Options{Rules: rules, Set: set, Strict: strict}
}

func toSet(s []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, k := range s {
		out[k] = struct{}{}
	}
	return out
}

func redactToMap(t *testing.T, in string, opts Options) map[string]any {
	t.Helper()
	var out bytes.Buffer
	if _, err := JSON(strings.NewReader(in), &out, opts); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out.String())
	}
	return m
}

func TestJSON_RedactKeyTakesPrecedence(t *testing.T) {
	opts := buildOpts(t, []string{"id"}, []string{"email"}, true)
	got := redactToMap(t, `{"id": "abc", "email": "john@acme.io"}`, opts)
	if got["id"] != "abc" {
		t.Errorf("safe key mangled: %v", got["id"])
	}
	if got["email"] != tokenRedactKey {
		t.Errorf("redact key not applied: %v", got["email"])
	}
}

func TestJSON_UnknownKey_StrictMode_Redacted(t *testing.T) {
	opts := buildOpts(t, []string{"id"}, []string{"email"}, true)
	got := redactToMap(t, `{"id": "abc", "surprise": "data"}`, opts)
	if got["surprise"] != tokenRedactUnknown {
		t.Errorf("unknown key not redacted in strict mode: %v", got["surprise"])
	}
}

func TestJSON_UnknownKey_PermissiveMode_PassesThrough(t *testing.T) {
	opts := buildOpts(t, []string{"id"}, []string{"email"}, false)
	got := redactToMap(t, `{"surprise": "plain", "leaky": "john@acme.io"}`, opts)
	if got["surprise"] != "plain" {
		t.Errorf("permissive mode should keep unknown string: %v", got["surprise"])
	}
	// Content regex must still run in permissive mode.
	if !strings.Contains(got["leaky"].(string), "[REDACTED:email]") {
		t.Errorf("content regex missing in permissive mode: %v", got["leaky"])
	}
}

func TestJSON_SafeKey_ContainerRecurses(t *testing.T) {
	opts := buildOpts(t, []string{"payload"}, []string{"email"}, true)
	got := redactToMap(t, `{"payload": {"email": "a@b.io", "id": "abc"}}`, opts)
	p := got["payload"].(map[string]any)
	if p["email"] != tokenRedactKey {
		t.Errorf("nested redact key missed: %v", p["email"])
	}
	// "id" nested is unknown at that level -> redacted in strict
	if p["id"] != tokenRedactUnknown {
		t.Errorf("nested unknown not redacted: %v", p["id"])
	}
}

func TestJSON_ContentRegex_InSafeFreeText(t *testing.T) {
	opts := buildOpts(t, []string{"message"}, nil, true)
	got := redactToMap(t, `{"message": "User john@acme.io failed from 10.0.0.5"}`, opts)
	msg := got["message"].(string)
	if strings.Contains(msg, "john@acme.io") || strings.Contains(msg, "10.0.0.5") {
		t.Errorf("content regex did not redact inside safe free-text field: %q", msg)
	}
}

func TestJSON_TopLevelArray_RedactsEachEntry(t *testing.T) {
	opts := buildOpts(t, []string{"id"}, []string{"email"}, true)
	var out bytes.Buffer
	_, err := JSON(strings.NewReader(`[{"id":"1","email":"a@b.io"},{"id":"2","email":"c@d.io"}]`), &out, opts)
	if err != nil {
		t.Fatal(err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		t.Fatal(err)
	}
	for _, e := range arr {
		if e["email"] != tokenRedactKey {
			t.Errorf("array element not redacted: %v", e)
		}
	}
}

func TestJSON_TruncatesLongSafeStrings(t *testing.T) {
	opts := buildOpts(t, []string{"message"}, nil, true)
	opts.Rules.MaxSafeStringLength = 20
	long := strings.Repeat("x", 100)
	got := redactToMap(t, `{"message": "`+long+`"}`, opts)
	msg := got["message"].(string)
	if !strings.Contains(msg, "…[+80 chars]") {
		t.Errorf("expected truncation marker, got %q", msg)
	}
}

func TestJSON_FailsOnMalformedInput(t *testing.T) {
	opts := buildOpts(t, nil, nil, true)
	var out bytes.Buffer
	_, err := JSON(strings.NewReader(`{not json`), &out, opts)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if out.Len() != 0 {
		t.Error("failed parse must not write partial output")
	}
}

func TestJSON_ServiceOverride_AppliedPerEntry(t *testing.T) {
	rules := &config.Ruleset{
		SafeKeys:     map[string]struct{}{"id": {}, "resource": {}, "labels": {}, "container_name": {}, "custom": {}},
		RedactKeys:   map[string]struct{}{"email": {}},
		ServiceField: "resource.labels.container_name",
		ServiceOverrides: map[string]config.ServiceRules{
			"ats": {RedactKeys: []string{"notes"}},
		},
	}
	set, _ := patterns.Compile(nil)
	opts := Options{
		Rules: rules, Set: set, Strict: true,
		ServiceOf: func(e any) string { return ExtractByPath(e, "resource.labels.container_name") },
	}
	input := `[
		{"id": "1", "notes": "secret", "resource": {"labels": {"container_name": "ats"}}},
		{"id": "2", "notes": "public", "resource": {"labels": {"container_name": "other"}}}
	]`
	var out bytes.Buffer
	if _, err := JSON(strings.NewReader(input), &out, opts); err != nil {
		t.Fatal(err)
	}
	var arr []map[string]any
	_ = json.Unmarshal(out.Bytes(), &arr)
	if arr[0]["notes"] != tokenRedactKey {
		t.Errorf("ats override did not redact notes: %v", arr[0])
	}
	// "other" service has no override; notes is unknown and strict => redacted unknown.
	if arr[1]["notes"] != tokenRedactUnknown {
		t.Errorf("other service notes should be unknown-redacted, got %v", arr[1])
	}
}

func TestExtractByPath(t *testing.T) {
	entry := map[string]any{
		"resource": map[string]any{
			"labels": map[string]any{"container_name": "ats"},
		},
	}
	if got := ExtractByPath(entry, "resource.labels.container_name"); got != "ats" {
		t.Errorf("want ats, got %q", got)
	}
	if got := ExtractByPath(entry, "resource.missing.field"); got != "" {
		t.Errorf("missing path should return empty, got %q", got)
	}
	if got := ExtractByPath(entry, ""); got != "" {
		t.Errorf("empty path should return empty, got %q", got)
	}
}
