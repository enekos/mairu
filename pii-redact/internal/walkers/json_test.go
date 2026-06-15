package walkers

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/enekos/mairu/pii-redact/internal/config"
	"github.com/enekos/mairu/pii-redact/internal/patterns"
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

func TestJSON_IDKey_ExactMatch_PrimitiveUntouched(t *testing.T) {
	opts := buildOpts(t, nil, nil, true)
	opts.Rules.IDKeys = &config.IDKeySet{}
	opts.Rules.IDKeys = mustIDSet("id", "*Id")
	// id with a UUID value — content regex would otherwise mangle it.
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	opts.Set, _ = patterns.Compile(map[string]string{
		"uuid": `\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`,
	})
	got := redactToMap(t, `{"id": "`+uuid+`"}`, opts)
	if got["id"] != uuid {
		t.Errorf("id_keys must preserve UUID primary key: got %v, want %s", got["id"], uuid)
	}
}

func TestJSON_IDKey_SuffixGlob_FKPreserved(t *testing.T) {
	opts := buildOpts(t, nil, nil, true)
	opts.Rules.IDKeys = mustIDSet("*Id", "*_id")
	got := redactToMap(t, `{"candidateId": "cand-42", "application_id": "app-9", "userId": 12345}`, opts)
	if got["candidateId"] != "cand-42" {
		t.Errorf("candidateId not preserved: %v", got["candidateId"])
	}
	if got["application_id"] != "app-9" {
		t.Errorf("application_id not preserved: %v", got["application_id"])
	}
	// json.Number round-trips through json.Marshal as a number.
	if n, ok := got["userId"].(float64); !ok || n != 12345 {
		t.Errorf("numeric userId not preserved: %v", got["userId"])
	}
}

func TestJSON_RedactKey_StillBeatsIDKey(t *testing.T) {
	// Precedence: redact_keys > id_keys. If a key is in both lists, redact wins.
	opts := buildOpts(t, nil, []string{"sessionId"}, true)
	opts.Rules.IDKeys = mustIDSet("*Id")
	got := redactToMap(t, `{"sessionId": "s-1", "candidateId": "c-1"}`, opts)
	if got["sessionId"] != tokenRedactKey {
		t.Errorf("redact_keys should win over id_keys: %v", got["sessionId"])
	}
	if got["candidateId"] != "c-1" {
		t.Errorf("candidateId should pass via *Id: %v", got["candidateId"])
	}
}

func TestJSON_IDKey_ContainerRecurses(t *testing.T) {
	// An id_keys container should still have nested PII redacted.
	opts := buildOpts(t, nil, []string{"email"}, true)
	opts.Rules.IDKeys = mustIDSet("*Refs")
	got := redactToMap(t, `{"candidateRefs": {"email": "a@b.io", "candidateId": "c-1"}}`, opts)
	inner := got["candidateRefs"].(map[string]any)
	if inner["email"] != tokenRedactKey {
		t.Errorf("nested email under id_keys container must still redact: %v", inner["email"])
	}
	if inner["candidateId"] != tokenRedactUnknown {
		t.Errorf("nested candidateId (no id_keys for it here) should be unknown in strict: %v", inner["candidateId"])
	}
}

func mustIDSet(patterns ...string) *config.IDKeySet {
	s := &config.IDKeySet{}
	for _, p := range patterns {
		// Mirror the internal additive parsing via the public Match path:
		// we can't call s.add (unexported) from here, but the parser is
		// shared with the loader, so reconstruct via direct fields.
		if len(p) > 1 && p[0] == '*' {
			s.Suffixes = append(s.Suffixes, p[1:])
		} else {
			if s.Exact == nil {
				s.Exact = map[string]struct{}{}
			}
			s.Exact[p] = struct{}{}
		}
	}
	return s
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
