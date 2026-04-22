// Package redact is the stable public API for the pii-redact engine.
//
// Two operating modes are available:
//
//   - Redactor.Redact(input, kind) runs the in-process layered pipeline on
//     a string and returns a Result. Use this for agent tool output, shell
//     history, free-text memory content, or any "scrub this one value"
//     flow.
//
//   - Redactor.RedactStream(in, out, kind) streams a log source through
//     the appropriate walker (JSON / NDJSON / logfmt / line). Use this
//     when reading stdin or a file and writing to stdout/file with the
//     full policy engine (safe_keys, redact_keys, per-service overrides).
//
// The public shape is deliberately small and concrete. Everything else
// lives under internal/ and may change without notice.
package redact

import (
	"fmt"
	"io"

	"github.com/enekos/mairu/pii-redact/internal/config"
	"github.com/enekos/mairu/pii-redact/internal/mask"
	"github.com/enekos/mairu/pii-redact/internal/patterns"
	"github.com/enekos/mairu/pii-redact/internal/pipeline"
	"github.com/enekos/mairu/pii-redact/internal/walkers"
)

// Kind routes an input to the right processing path.
type Kind int

const (
	// KindText runs Layers 0-4 + damage cap on free-form text.
	KindText Kind = iota
	// KindCommand runs the same layers as KindText plus the Layer-5
	// command denylist (vault kv read, op read, gcloud auth, ...).
	KindCommand
	// KindJSON runs the structured JSON walker and applies the per-string
	// text pipeline to every surviving string leaf.
	KindJSON
	// KindNDJSON is KindJSON with one object per line.
	KindNDJSON
	// KindLogfmt runs the logfmt key=value walker.
	KindLogfmt
	// KindLine runs the per-line content-pattern pass.
	KindLine
)

// Layer identifies which redaction stage produced a Finding.
type Layer = pipeline.Layer

const (
	LayerFormat     = pipeline.LayerFormat
	LayerKnownToken = pipeline.LayerKnownToken
	LayerArgFlag    = pipeline.LayerArgFlag
	LayerFreeText   = pipeline.LayerFreeText
	LayerEntropy    = pipeline.LayerEntropy
	LayerDenylist   = pipeline.LayerDenylist
	LayerDamageCap  = pipeline.LayerDamageCap
)

// Finding records one redaction event: which layer, what kind of secret,
// and the offsets in the original input where it was detected.
type Finding = pipeline.Finding

// Result is the output of Redact.
type Result struct {
	Redacted      string
	Findings      []Finding
	EmbeddingSafe bool
	Dropped       bool
	Stats         map[string]int
}

// Options configures a Redactor. Zero values pick sensible defaults.
type Options struct {
	// Reveal selects partial-reveal masking (default) vs. opaque
	// [REDACTED:<name>] replacement. Only affects KindJSON/NDJSON/Logfmt/Line.
	Reveal bool
	// Strict controls JSON-walker behavior: when true (default) unknown
	// keys are redacted; when false they pass through with only content
	// patterns applied.
	Strict bool
	// EntropyThreshold is the Shannon bits/byte cutoff for Layer 4.
	// Default 4.5. Lower = more aggressive.
	EntropyThreshold float64
	// DamageCap is the maximum fraction of the output that may be inside
	// [REDACTED:...] placeholders. Above that, the record is collapsed.
	// Default 0.5. Set <=0 to disable.
	DamageCap float64
	// MinEntropyLen skips short candidates below this length. Default 20.
	MinEntropyLen int
	// DenylistCommands lists program names that, when paired with a risky
	// subcommand, cause Layer 5 to collapse the argv. Default includes
	// vault, op, pass, gpg, aws, gh, doctl, gcloud, kubectl.
	DenylistCommands []string
	// SkipFormatLayer disables the Layer-0 format-aware pre-pass. Useful
	// when the caller already knows the input is a single value.
	SkipFormatLayer bool
	// Rules is an optional key/pattern ruleset for the JSON walker. Leave
	// nil for text/command use.
	Rules *config.Ruleset
	// Profile is a bundled profile name to load (e.g. "gcp-logging").
	// Only consulted if Rules is nil. "" means no profile.
	Profile string
	// ConfigDirs / Configs are forwarded to config.Load when Rules is nil.
	ConfigDirs []string
	Configs    []string
}

// Redactor is a prepared redaction engine. Build once, reuse across
// calls. It is safe for concurrent use — underlying regex tables and
// masker are read-only after construction.
type Redactor struct {
	pipeOpts pipeline.Options
	rules    *config.Ruleset
	set      *patterns.Set
	masker   *mask.Masker
	strict   bool
}

// New builds a Redactor from the given Options.
func New(opts Options) (*Redactor, error) {
	po := pipeline.DefaultOptions()
	if opts.EntropyThreshold > 0 {
		po.EntropyThreshold = opts.EntropyThreshold
	}
	if opts.DamageCap > 0 {
		po.DamageCapRatio = opts.DamageCap
	}
	if opts.MinEntropyLen > 0 {
		po.MinEntropyLen = opts.MinEntropyLen
	}
	if len(opts.DenylistCommands) > 0 {
		po.DenylistCommands = opts.DenylistCommands
	}
	po.SkipFormat = opts.SkipFormatLayer

	r := &Redactor{pipeOpts: po, strict: opts.Strict}

	rules := opts.Rules
	if rules == nil && (opts.Profile != "" || len(opts.ConfigDirs) > 0 || len(opts.Configs) > 0) {
		loaded, err := config.Load(config.LoadOptions{
			Profile:    opts.Profile,
			ConfigDirs: opts.ConfigDirs,
			Configs:    opts.Configs,
		})
		if err != nil {
			return nil, fmt.Errorf("load rules: %w", err)
		}
		rules = loaded
	}
	if rules != nil {
		set, err := patterns.Compile(rules.ContentPatterns)
		if err != nil {
			return nil, fmt.Errorf("compile patterns: %w", err)
		}
		masker := mask.NewMasker(opts.Reveal)
		set = set.WithMasker(masker)
		r.rules = rules
		r.set = set
		r.masker = masker
	}

	return r, nil
}

// Redact runs the layered pipeline on a single string.
//
// For KindText / KindCommand: returns the fully-redacted string with
// findings and the EmbeddingSafe flag (false if any Layer-1 known-token
// pattern hit).
//
// For KindJSON / KindNDJSON / KindLogfmt / KindLine: use RedactStream
// instead — Redact refuses these kinds because they require an io.Reader
// and return a non-string framing.
func (r *Redactor) Redact(input string, kind Kind) Result {
	switch kind {
	case KindText, KindCommand:
		pk := pipeline.KindText
		if kind == KindCommand {
			pk = pipeline.KindCommand
		}
		pr := pipeline.Run(input, pk, r.pipeOpts)
		return Result{
			Redacted:      pr.Redacted,
			Findings:      pr.Findings,
			EmbeddingSafe: pr.EmbeddingSafe,
			Dropped:       pr.Dropped,
		}
	default:
		return Result{
			Redacted:      "",
			EmbeddingSafe: false,
			Dropped:       true,
		}
	}
}

// RedactStream processes an io.Reader through the walker appropriate for
// kind, writing the result to out. Stats are aggregated across the run.
func (r *Redactor) RedactStream(in io.Reader, out io.Writer, kind Kind) (map[string]int, error) {
	if r.set == nil {
		return nil, fmt.Errorf("redact: stream mode requires Rules/Profile/Config options")
	}
	opts := walkers.Options{
		Rules:  r.rules,
		Set:    r.set,
		Masker: r.masker,
		Strict: r.strict,
	}
	if r.rules != nil && r.rules.ServiceField != "" {
		path := r.rules.ServiceField
		opts.ServiceOf = func(entry any) string {
			return walkers.ExtractByPath(entry, path)
		}
	}

	switch kind {
	case KindJSON:
		return walkers.JSON(in, out, opts)
	case KindNDJSON:
		return walkers.NDJSON(in, out, opts)
	case KindLogfmt:
		return walkers.Logfmt(in, out, opts)
	case KindLine:
		return walkers.Lines(in, out, r.set)
	default:
		return nil, fmt.Errorf("redact: unsupported stream kind %d", kind)
	}
}
