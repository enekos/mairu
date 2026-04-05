package tui

import "time"

var quirkySpinnerFrames = []string{
	"◜", "◝", "◞", "◟", "◠", "◡", "✶", "✷", "✸", "✹", "✺",
}

var xiberokoLoadingPhrases = []string{
	"Konbobulatzen...",
	"Arranjatzen...",
	"Xerkatzen...",
	"Bürütatzen...",
	"Aitzinatzen...",
	"Üngürükatzen...",
	"Moldatzen...",
	"Eztiki nahasten...",
	"Xuxentzen...",
	"Berriz antolatzen...",
	"Trikimailatzen...",
	"Biribilkatzen...",
	"Ñabarduratzen...",
	"Xorroxtatzen...",
	"Leunki orrazten...",
	"Maskaratzen...",
	"Hitzak xuxurlatzen...",
	"Harilkatuz pentsatzen...",
	"Astiro moldagaitzen...",
	"Bihurrikatzen...",
	"Xedetan trebatzen...",
	"Zirtzilatzen...",
	"Bapatez argitzen...",
	"Isilik josten...",
	"Ageri-ezkutu lanean...",
}

func (m *model) refreshThinkingIndicator(now time.Time, forcePhrase bool) {
	if m.rng == nil {
		return
	}

	// Keep the glyph lively and unpredictable while streaming.
	if m.thinkingGlyph == "" || m.rng.Intn(100) < 72 {
		m.thinkingGlyph = quirkySpinnerFrames[m.rng.Intn(len(quirkySpinnerFrames))]
	}

	if forcePhrase || m.thinkingPhrase == "" || now.After(m.nextPhraseSwitchAt) {
		m.thinkingPhrase = xiberokoLoadingPhrases[m.rng.Intn(len(xiberokoLoadingPhrases))]
		nextMs := 800 + m.rng.Intn(1000) // rotation
		m.nextPhraseSwitchAt = now.Add(time.Duration(nextMs) * time.Millisecond)
	}
}

func (m *model) clearThinkingIndicator() {
	m.thinkingGlyph = ""
	m.thinkingPhrase = ""
	m.nextPhraseSwitchAt = time.Time{}
}
