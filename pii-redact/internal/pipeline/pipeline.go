// Package pipeline runs the ordered redaction layers for free-form text and
// shell commands. It is the shared engine behind the public pkg/redact API
// and the JSON-walker-applied per-string pipeline.
//
// Layer ordering is fixed:
//
//  0. Format-aware pre-pass (.env / YAML / connection strings / headers)
//  1. Known-token regex (provider-specific credentials)
//  2. Arg/flag heuristics (-H Authorization: ..., --token=, ENV=, -u user:pass)
//  3. Free-text PII (email / phone / IBAN / CC / SSN / public IPv4)
//  4. Shannon entropy (high-entropy long strings)
//  5. Command denylist (vault kv, op read, gcloud auth)
//  6. Damage cap (collapse if too much was redacted)
//
// Only Layer 1 flips EmbeddingSafe=false. Other layers may match without
// clearing that bit: they signal "scrubbed but likely still safe to embed."
package pipeline

type Kind int

const (
	KindText Kind = iota
	KindCommand
)

type Layer int

const (
	LayerFormat     Layer = 0
	LayerKnownToken Layer = 1
	LayerArgFlag    Layer = 2
	LayerFreeText   Layer = 3
	LayerEntropy    Layer = 4
	LayerDenylist   Layer = 5
	LayerDamageCap  Layer = 6
)

type Finding struct {
	Layer Layer
	Kind  string
	Start int
	End   int
}

type Result struct {
	Redacted      string
	Findings      []Finding
	EmbeddingSafe bool
	Dropped       bool
}

type Options struct {
	DenylistCommands []string
	EntropyThreshold float64
	DamageCapRatio   float64
	MinEntropyLen    int
	// SkipFormat disables the format-aware pre-pass. Useful when the caller
	// has already normalized input or when the input is guaranteed to be a
	// single value rather than a multi-line document.
	SkipFormat bool
}

func DefaultOptions() Options {
	return Options{
		DenylistCommands: defaultDenylistCommands(),
		EntropyThreshold: 4.5,
		DamageCapRatio:   0.5,
		MinEntropyLen:    20,
	}
}

// Run applies all layers in order and returns the redacted output plus the
// findings accumulated along the way. It is safe for concurrent use: the
// regex tables it consults are read-only package-level state.
func Run(input string, kind Kind, opts Options) (res Result) {
	defer func() {
		if rec := recover(); rec != nil {
			res = Result{
				Redacted:      "[REDACTED:panic]",
				EmbeddingSafe: false,
				Dropped:       true,
			}
		}
	}()

	current := input
	findings := make([]Finding, 0, 4)
	embeddingSafe := true

	if !opts.SkipFormat {
		var l0 []Finding
		current, l0 = scanFormats(current)
		findings = append(findings, l0...)
	}

	{
		cleaned, l1, hit := scanKnownTokens(current)
		current = cleaned
		findings = append(findings, l1...)
		if hit {
			embeddingSafe = false
		}
	}

	if kind == KindCommand {
		cleaned, l2 := scanArguments(current)
		current = cleaned
		findings = append(findings, l2...)
	}

	{
		cleaned, l3 := scanFreeText(current)
		current = cleaned
		findings = append(findings, l3...)
	}

	{
		cleaned, l4 := scanEntropy(current, opts.EntropyThreshold, opts.MinEntropyLen)
		current = cleaned
		findings = append(findings, l4...)
	}

	if kind == KindCommand {
		cleaned, l5 := scanCommandDenylist(current, opts.DenylistCommands)
		current = cleaned
		findings = append(findings, l5...)
	}

	capped, dropped := applyDamageCap(current, kind, opts.DamageCapRatio)
	current = capped
	if dropped {
		findings = append(findings, Finding{Layer: LayerDamageCap, Kind: "damage_cap"})
	}

	return Result{
		Redacted:      current,
		Findings:      findings,
		EmbeddingSafe: embeddingSafe,
		Dropped:       dropped,
	}
}
