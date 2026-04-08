package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra/doc"
)

func GenerateDocs() {
	err := os.MkdirAll("docs/cli", 0755)
	if err != nil {
		log.Fatal(err)
	}
	err = doc.GenMarkdownTree(rootCmd, "docs/cli")
	if err != nil {
		log.Fatal(err)
	}
}
