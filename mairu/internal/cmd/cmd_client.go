package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func contextServerURL() string {
	base := strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_URL"))
	return strings.TrimRight(base, "/")
}

func contextToken() string {
	return strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_TOKEN"))
}

func ContextGet(path string, params map[string]string) ([]byte, error) {
	baseURL := contextServerURL()
	if baseURL == "" {
		baseURL = "http://localhost" // placeholder for local routing
	}
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range params {
		if strings.TrimSpace(v) == "" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if tok := contextToken(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return doContextRequest(req)
}

func ContextPost(path string, payload any) ([]byte, error) {
	baseURL := contextServerURL()
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := contextToken(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return doContextRequest(req)
}

func ContextPut(path string, payload any) ([]byte, error) {
	baseURL := contextServerURL()
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := contextToken(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return doContextRequest(req)
}

func ContextDelete(path string, params map[string]string) ([]byte, error) {
	baseURL := contextServerURL()
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range params {
		if strings.TrimSpace(v) == "" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if tok := contextToken(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return doContextRequest(req)
}

func doContextRequest(req *http.Request) ([]byte, error) {
	if contextServerURL() == "" {
		// Use local in-memory handler
		localHandler := getLocalHandler()
		if localHandler == nil {
			return nil, fmt.Errorf("local context service could not be initialized")
		}

		rec := httptest.NewRecorder()
		localHandler.ServeHTTP(rec, req)

		resp := rec.Result()
		body := rec.Body.Bytes()

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("context server HTTP %d: %s", resp.StatusCode, string(body))
		}
		return body, nil
	}

	// Remote request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("context server error: %v", err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	body := buf.Bytes()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("context server HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func PrintJSON(raw []byte) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Println(string(raw))
		return
	}
	formatted, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(string(raw))
		return
	}
	fmt.Println(string(formatted))
}

func AddCommonSearchFlags(cmd *cobra.Command) {
	cmd.Flags().IntP("topK", "k", 5, "Number of results to return")
	cmd.Flags().Float64("minScore", 0, "Minimum relevance score threshold")
	cmd.Flags().Bool("highlight", false, "Include text highlights in response")
	cmd.Flags().Int("fuzziness", -1, "Typo tolerance (0-2, or -1 for auto)")
	cmd.Flags().Float64("phraseBoost", 0, "Boost phrase exact matches (e.g. 3.0)")
	cmd.Flags().String("owner", "", "Filter by owner")
	cmd.Flags().String("category", "", "Filter by category")
	cmd.Flags().String("from", "", "Filter by creation date from (e.g. 2024-01-01)")
	cmd.Flags().String("to", "", "Filter by creation date to (e.g. 2024-12-31)")
}

func SearchParamsFromFlags(cmd *cobra.Command, query, store, project string) map[string]string {
	k, _ := cmd.Flags().GetInt("topK")
	minScore, _ := cmd.Flags().GetFloat64("minScore")
	highlight, _ := cmd.Flags().GetBool("highlight")
	fuzziness, _ := cmd.Flags().GetInt("fuzziness")
	phraseBoost, _ := cmd.Flags().GetFloat64("phraseBoost")
	owner, _ := cmd.Flags().GetString("owner")
	category, _ := cmd.Flags().GetString("category")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")

	params := map[string]string{
		"q":       query,
		"type":    store,
		"project": project,
		"topK":    fmt.Sprintf("%d", k),
	}
	if minScore > 0 {
		params["minScore"] = fmt.Sprintf("%v", minScore)
	}
	if highlight {
		params["highlight"] = "true"
	}
	if fuzziness >= 0 {
		params["fuzziness"] = fmt.Sprintf("%d", fuzziness)
	}
	if phraseBoost > 0 {
		params["phraseBoost"] = fmt.Sprintf("%v", phraseBoost)
	}
	if owner != "" {
		params["owner"] = owner
	}
	if category != "" {
		params["category"] = category
	}
	if from != "" {
		params["from"] = from
	}
	if to != "" {
		params["to"] = to
	}
	return params
}

func Truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
