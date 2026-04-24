package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"mairu/internal/ctxclient"

	"github.com/spf13/cobra"
)

func ContextGet(path string, params map[string]string) ([]byte, error) {
	return contextRequest(http.MethodGet, path, params, nil)
}

func ContextPost(path string, payload any) ([]byte, error) {
	return contextRequest(http.MethodPost, path, nil, payload)
}

func ContextPut(path string, payload any) ([]byte, error) {
	return contextRequest(http.MethodPut, path, nil, payload)
}

func ContextDelete(path string, params map[string]string) ([]byte, error) {
	return contextRequest(http.MethodDelete, path, params, nil)
}

func contextRequest(method, path string, params map[string]string, body any) ([]byte, error) {
	base := ctxclient.BaseURL()
	target := base
	if target == "" {
		target = "http://localhost"
	}
	req, err := ctxclient.Build(method, target+path, params, body)
	if err != nil {
		return nil, err
	}
	if base == "" {
		return serveLocal(req)
	}
	return ctxclient.Do(req)
}

func serveLocal(req *http.Request) ([]byte, error) {
	h := getLocalHandler()
	if h == nil {
		return nil, fmt.Errorf("local context service could not be initialized")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	resp := rec.Result()
	body := rec.Body.Bytes()
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

func addProjectFlag(c *cobra.Command, dest *string) {
	c.PersistentFlags().StringVarP(dest, "project", "P", "", "Project name")
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
