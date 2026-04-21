package approved

import (
	"os"
	"path/filepath"
	"strings"
)

// FixtureRule defines a convention: input files matching InputGlob
// must have a corresponding approved file derived by MapFunc.
type FixtureRule struct {
	InputGlob string
	MapFunc   func(inputPath string) string
}

// CheckFixtures verifies that every input file matching the rules
// has a corresponding approved file on disk. Returns missing paths.
func CheckFixtures(rules ...FixtureRule) (missing []string, err error) {
	for _, rule := range rules {
		inputs, err := DiscoverInputs(rule.InputGlob)
		if err != nil {
			return nil, err
		}
		for _, input := range inputs {
			approved := rule.MapFunc(input)
			if _, err := os.Stat(approved); os.IsNotExist(err) {
				missing = append(missing, approved)
			}
		}
	}
	return missing, nil
}

// DiscoverInputs returns all file paths matching the given glob pattern.
func DiscoverInputs(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// MapInputToApprovedJSON converts an input file path to its approved JSON path.
// "testdata/ts/foo.input.ts" -> "testdata/ts/foo.approved.json"
func MapInputToApprovedJSON(inputPath string) string {
	dir := filepath.Dir(inputPath)
	base := filepath.Base(inputPath)
	parts := strings.SplitN(base, ".input.", 2)
	if len(parts) != 2 {
		return inputPath + ".approved.json"
	}
	return filepath.Join(dir, parts[0]+".approved.json")
}

// MapInputToApprovedMD converts an input file path to its approved markdown path.
// "testdata/nl/foo.input.ts" -> "testdata/nl/foo.approved.md"
func MapInputToApprovedMD(inputPath string) string {
	dir := filepath.Dir(inputPath)
	base := filepath.Base(inputPath)
	parts := strings.SplitN(base, ".input.", 2)
	if len(parts) != 2 {
		return inputPath + ".approved.md"
	}
	return filepath.Join(dir, parts[0]+".approved.md")
}
