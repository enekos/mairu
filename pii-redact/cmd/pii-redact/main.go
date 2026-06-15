// Command pii-redact streams log data from stdin to stdout with PII removed.
// See the repo README for the CLI contract.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/enekos/mairu/pii-redact/internal/config"
	"github.com/enekos/mairu/pii-redact/internal/mask"
	"github.com/enekos/mairu/pii-redact/internal/patterns"
	walkers "github.com/enekos/mairu/pii-redact/internal/walkers"
)

const version = "0.2.0"

const (
	exitOK          = 0
	exitConfigError = 1
	exitParseError  = 2
	exitIOError     = 3
)

// stringSlice supports repeatable flag values.
type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

type cliFlags struct {
	mode         string
	configDirs   stringSlice
	configs      stringSlice
	profile      string
	serviceField string
	strict       bool
	permissive   bool
	stats        bool
	quiet        bool
	reveal       bool
	opaque       bool
	keepIDs      bool
	showVersion  bool
}

// defaultIDKeys is the broad set applied when --keep-ids is on, so common
// DB primary/foreign-key columns (id, *Id, *_id, *Uuid, …) and tracing IDs
// pass through untouched even if the active profile omits them.
var defaultIDKeys = []string{
	"id", "traceId", "spanId", "insertId",
	"*Id", "*_id", "*Uuid", "*_uuid", "*UUID", "*_UUID",
}

func main() {
	fs := flag.NewFlagSet("pii-redact", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f cliFlags
	fs.StringVar(&f.mode, "mode", "auto", "input mode: auto|json|line|ndjson|logfmt|sql")
	fs.Var(&f.configDirs, "config-dir", "directory with global.json + services/*.json (repeatable)")
	fs.Var(&f.configs, "config", "individual config file (repeatable)")
	fs.StringVar(&f.profile, "profile", "", "bundled profile to enable (e.g. gcp-logging)")
	fs.StringVar(&f.serviceField, "service-field", "", "JSON path used to pick per-service override (overrides profile/config value)")
	fs.BoolVar(&f.strict, "strict", true, "redact keys not on either list (default)")
	fs.BoolVar(&f.permissive, "permissive", false, "let unknown keys pass through (content regex still runs)")
	fs.BoolVar(&f.stats, "stats", false, "emit redaction histogram to stderr before exit")
	fs.BoolVar(&f.quiet, "quiet", false, "suppress non-fatal warnings")
	fs.BoolVar(&f.reveal, "reveal", true, "partial-reveal masking (default): keeps tails/prefixes so entries remain distinguishable")
	fs.BoolVar(&f.opaque, "opaque", false, "disable partial-reveal; every match rendered as [REDACTED:<name>]")
	fs.BoolVar(&f.keepIDs, "keep-ids", false, "skip ID redaction: drop uuid content pattern + broaden id_keys to *Id/*_id/*Uuid/*UUID so DB query output survives intact")
	fs.BoolVar(&f.showVersion, "version", false, "print version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		// flag already printed usage on stderr.
		os.Exit(exitConfigError)
	}

	if f.showVersion {
		fmt.Println("pii-redact", version)
		os.Exit(exitOK)
	}

	code, err := run(f, os.Stdin, os.Stdout, os.Stderr)
	if err != nil && !f.quiet {
		fmt.Fprintf(os.Stderr, "pii-redact: %v\n", err)
	}
	os.Exit(code)
}

// run is the testable entrypoint. It buffers all output and only flushes
// to `out` on success — fail-closed guarantee.
func run(f cliFlags, in io.Reader, out, errOut io.Writer) (int, error) {
	rules, err := config.Load(config.LoadOptions{
		Profile:    f.profile,
		ConfigDirs: []string(f.configDirs),
		Configs:    []string(f.configs),
	})
	if err != nil {
		return exitConfigError, err
	}
	if f.serviceField != "" {
		rules.ServiceField = f.serviceField
	}

	if f.keepIDs {
		delete(rules.ContentPatterns, "uuid")
		for _, k := range defaultIDKeys {
			rules.IDKeys.Add(k)
		}
	}

	set, err := patterns.Compile(rules.ContentPatterns)
	if err != nil {
		return exitConfigError, err
	}
	reveal := f.reveal && !f.opaque
	masker := mask.NewMasker(reveal)
	set = set.WithMasker(masker)

	strict := f.strict && !f.permissive

	mode := f.mode
	var buffered bytes.Buffer
	var stats patterns.Stats

	switch mode {
	case "auto":
		peeked, reader, err := peekMode(in)
		if err != nil {
			return exitIOError, err
		}
		mode = peeked
		in = reader
	}

	switch mode {
	case "json":
		opts := buildJSONOpts(rules, set, masker, strict)
		stats, err = walkers.JSON(in, &buffered, opts)
		if err != nil {
			return exitParseError, err
		}
	case "ndjson":
		opts := buildJSONOpts(rules, set, masker, strict)
		stats, err = walkers.NDJSON(in, &buffered, opts)
		if err != nil {
			return exitParseError, err
		}
	case "logfmt":
		opts := buildJSONOpts(rules, set, masker, strict)
		stats, err = walkers.Logfmt(in, &buffered, opts)
		if err != nil {
			return exitIOError, err
		}
	case "line":
		stats, err = walkers.Lines(in, &buffered, set)
		if err != nil {
			return exitIOError, err
		}
	case "sql":
		opts := buildJSONOpts(rules, set, masker, strict)
		stats, err = walkers.SQL(in, &buffered, opts)
		if err != nil {
			return exitIOError, err
		}
	default:
		return exitConfigError, fmt.Errorf("unknown --mode %q (want auto|json|line|ndjson|logfmt|sql)", mode)
	}

	// Flush buffered, fully-redacted output to real stdout only now.
	if _, err := io.Copy(out, &buffered); err != nil {
		return exitIOError, err
	}

	if f.stats {
		writeStats(errOut, stats)
	}
	return exitOK, nil
}

func buildJSONOpts(rules *config.Ruleset, set *patterns.Set, masker *mask.Masker, strict bool) walkers.Options {
	opts := walkers.Options{Rules: rules, Set: set, Masker: masker, Strict: strict}
	if rules.ServiceField != "" {
		path := rules.ServiceField
		opts.ServiceOf = func(entry any) string {
			return walkers.ExtractByPath(entry, path)
		}
	}
	return opts
}

// peekMode inspects the first non-whitespace byte to decide json vs line.
// Returns a new reader that replays the consumed bytes.
func peekMode(in io.Reader) (string, io.Reader, error) {
	br := bufio.NewReader(in)
	for {
		b, err := br.Peek(1)
		if errors.Is(err, io.EOF) {
			return "line", br, nil
		}
		if err != nil {
			return "", nil, err
		}
		c := b[0]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			if _, err := br.Discard(1); err != nil {
				return "", nil, err
			}
			continue
		}
		if c == '[' || c == '{' {
			return "json", br, nil
		}
		return "line", br, nil
	}
}

func writeStats(w io.Writer, stats patterns.Stats) {
	type kv struct {
		K string
		V int
	}
	list := make([]kv, 0, len(stats))
	for k, v := range stats {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].V > list[j].V })

	payload := map[string]int{}
	for _, e := range list {
		payload[e.K] = e.V
	}
	b, _ := json.Marshal(payload)
	fmt.Fprintf(w, "pii-redact stats: %s\n", string(b))
}
