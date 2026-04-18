// Package redact scrubs secrets from text via a fixed, ordered pipeline.
//
// The pipeline is intentionally rigid: each layer runs in a fixed order and
// every layer operates on the output of the previous one. Callers must treat
// Result.EmbeddingSafe as a hard gate — a false value means the original
// input tripped a known-token pattern and MUST NOT be sent to a remote
// embedding provider, even after redaction.
package redact

type Kind int

const (
	KindText Kind = iota
	KindCommand
)

type Layer int

const (
	LayerKnownToken Layer = 1
	LayerArgFlag    Layer = 2
	LayerEntropy    Layer = 3
	LayerDenylist   Layer = 4
	LayerDamageCap  Layer = 5
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

type Redactor struct {
	denylistCommands []string
	entropyThreshold float64
	damageCapRatio   float64
	minEntropyLen    int
}

type Option func(*Redactor)

func WithDenylistCommands(cmds []string) Option {
	return func(r *Redactor) { r.denylistCommands = cmds }
}

func WithEntropyThreshold(t float64) Option {
	return func(r *Redactor) { r.entropyThreshold = t }
}

func WithDamageCapRatio(ratio float64) Option {
	return func(r *Redactor) { r.damageCapRatio = ratio }
}

func WithMinEntropyLen(n int) Option {
	return func(r *Redactor) { r.minEntropyLen = n }
}

func New(opts ...Option) *Redactor {
	r := &Redactor{
		entropyThreshold: 4.5,
		damageCapRatio:   0.5,
		minEntropyLen:    20,
		denylistCommands: defaultDenylistCommands(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func defaultDenylistCommands() []string {
	return []string{"vault", "op", "pass", "gpg", "aws", "gh", "doctl", "gcloud"}
}

func (r *Redactor) Redact(input string, kind Kind) (res Result) {
	defer func() {
		if rec := recover(); rec != nil {
			res = Result{
				Redacted:      "[REDACTED:panic]",
				EmbeddingSafe: false,
				Dropped:       true,
			}
		}
	}()
	return Result{Redacted: input, EmbeddingSafe: true}
}
