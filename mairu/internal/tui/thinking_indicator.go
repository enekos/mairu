package tui

import "time"

var quirkySpinnerFrames = []string{
	"◴", "◷", "◶", "◵", "✶", "✸", "✺", "✸", "✶",
}

var xiberokoLoadingPhrases = []string{
	"Igürikatzen...",
	"Eijerki apailatzen...",
	"Mustrakan ari...",
	"Hitzak txarrantxatzen...",
	"Bürüa khilikatzen...",
	"Zühürtziaz ehuntzen...",
	"Bürü-hausterietan...",
	"Egiari hüllantzen...",
	"Aitzindarien urratsetan...",
	"Sükhalteko süan txigortzen...",
	"Mündia iraulikatzen...",
	"Satanen pheredikia asmatzen...",
	"Khordokak xuxentzen...",
	"Ülünpetik argitara jalkitzen...",
	"Düdak lürruntzen...",
	"Erran-zaharrak marraskatzen...",
	"Khexatü gabe phentsatzen...",
	"Ahapetik xuxurlatzen...",
	"Bortüetako haizea behatzen...",
	"Gogoa eküratzen...",
	"Orhoikizünak xahatzen...",
	"Belagileen artean...",
	"Ilhintiak phizten...",
	"Xühürki barnebistatzen...",
	"Errejent gisa moldatzen...",
	"Basa-ahaideak asmatzen...",
	"Zamaltzainaren jauzia prestatzen...",
	"Txülülen hotsari behatzen...",
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
