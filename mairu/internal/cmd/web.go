package cmd

import (
	"fmt"
	"mairu/internal/web"
	"os"

	"github.com/spf13/cobra"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the Mairu web interface",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		meiliURL, _ := cmd.Flags().GetString("meili-url")
		meiliAPIKey, _ := cmd.Flags().GetString("meili-api-key")
		apiKey := GetAPIKey()
		if apiKey == "" {
			fmt.Println("Error: Gemini API key not found. Please run 'mairu setup' or set GEMINI_API_KEY environment variable.")
			os.Exit(1)
		}
		fmt.Printf("Starting Mairu web interface on port %d...\n", port)
		if err := web.StartServer(port, apiKey, meiliURL, meiliAPIKey); err != nil {
			fmt.Printf("Error starting web server: %v\n", err)
		}
	},
}

func init() {
	webCmd.Flags().IntP("port", "p", 8080, "Port to run the web server on")
	webCmd.Flags().String("meili-url", os.Getenv("MEILI_URL"), "Meilisearch URL")
	webCmd.Flags().String("meili-api-key", os.Getenv("MEILI_API_KEY"), "Meilisearch API key")
	rootCmd.AddCommand(webCmd)
}
