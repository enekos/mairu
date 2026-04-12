package cmd

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"mairu/internal/agent"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	tele "gopkg.in/telebot.v3"
)

func formatTelegramHTML(md string) string {
	res := html.EscapeString(md)

	// Code blocks
	reCodeBlock := regexp.MustCompile("(?s)```(?:[a-zA-Z0-9]+)?\n?(.*?)```")
	res = reCodeBlock.ReplaceAllString(res, "<pre><code>$1</code></pre>")

	// Inline code
	reInlineCode := regexp.MustCompile("`([^`]+)`")
	res = reInlineCode.ReplaceAllString(res, "<code>$1</code>")

	// Bold
	reBold := regexp.MustCompile(`\*\*(.*?)\*\*`)
	res = reBold.ReplaceAllString(res, "<b>$1</b>")

	// Italic
	reItalic := regexp.MustCompile(`\*([^\*]+)\*`)
	res = reItalic.ReplaceAllString(res, "<i>$1</i>")

	return res
}

func sendLongMessage(c tele.Context, text string) error {
	lines := strings.Split(text, "\n")
	var chunk string

	sendChunk := func(msg string) error {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			return nil
		}
		htmlMsg := formatTelegramHTML(msg)
		err := c.Send(htmlMsg, &tele.SendOptions{ParseMode: tele.ModeHTML})
		if err != nil {
			slog.Error("HTML send failed, falling back to plain text", "error", err)
			return c.Send(msg)
		}
		return nil
	}

	for _, line := range lines {
		for len(line) > 4000 {
			if len(chunk) > 0 {
				sendChunk(chunk)
				chunk = ""
			}
			sendChunk(line[:4000])
			line = line[4000:]
		}

		if len(chunk)+len(line)+1 > 4000 {
			sendChunk(chunk)
			chunk = line + "\n"
		} else {
			chunk += line + "\n"
		}
	}

	if chunk != "" {
		sendChunk(chunk)
	}
	return nil
}

func NewTelegramCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telegram",
		Short: "Start Telegram bot interface",
		RunE: func(cmd *cobra.Command, args []string) error {
			token := os.Getenv("TELEGRAM_BOT_TOKEN")
			if token == "" {
				return fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required")
			}

			providerCfg := GetLLMProviderConfig()
			if providerCfg.APIKey == "" {
				providerName := providerCfg.Type
				if providerName == "" {
					providerName = "gemini"
				}
				return fmt.Errorf("%s API key not found. Please set the appropriate API key environment variable", providerName)
			}

			projectRoot, _ := cmd.Flags().GetString("project")
			if projectRoot == "" {
				pwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get current directory: %w", err)
				}
				projectRoot = pwd
			}

			allowedUsersRaw, _ := cmd.Flags().GetString("allowed-users")
			if allowedUsersRaw == "" {
				allowedUsersRaw = os.Getenv("TELEGRAM_ALLOWED_USERS")
			}

			allowedUsers := make(map[string]bool)
			for _, u := range strings.Split(allowedUsersRaw, ",") {
				u = strings.TrimSpace(u)
				if u != "" {
					allowedUsers[u] = true
				}
			}

			pref := tele.Settings{
				Token:  token,
				Poller: &tele.LongPoller{Timeout: 10 * time.Second},
			}

			b, err := tele.NewBot(pref)
			if err != nil {
				return fmt.Errorf("failed to create telegram bot: %w", err)
			}

			activeSessions := make(map[int64]string)

			b.Handle("/help", func(c tele.Context) error {
				helpText := `<b>Available Commands:</b>
- /help: Show this help message
- /clear: Clear the terminal context and session
- /save: Save the current session
- /compact: Compact the current session history
- /session [name]: Switch or create a named session
- !cmd: Run a local bash command and append its output to your prompt
- !!cmd: Run a local bash command immediately (output returned to you)
- @file/path: Include file contents in your prompt`
				return c.Send(helpText, &tele.SendOptions{ParseMode: tele.ModeHTML})
			})

			b.Handle("/session", func(c tele.Context) error {
				args := c.Args()
				if len(args) == 0 {
					current := activeSessions[c.Chat().ID]
					if current == "" {
						current = "default"
					}
					return c.Send(fmt.Sprintf("Current session: <b>%s</b>\nUsage: /session [name]", current), &tele.SendOptions{ParseMode: tele.ModeHTML})
				}
				activeSessions[c.Chat().ID] = args[0]
				return c.Send(fmt.Sprintf("Switched to session: <b>%s</b>", args[0]), &tele.SendOptions{ParseMode: tele.ModeHTML})
			})

			b.Handle("/clear", func(c tele.Context) error {
				sessionBase := activeSessions[c.Chat().ID]
				if sessionBase == "" {
					sessionBase = "default"
				}
				sessionName := fmt.Sprintf("tg-%d-%s", c.Chat().ID, sessionBase)

				ag, err := agent.New(projectRoot, providerCfg, agent.Config{
					SymbolLocator: GetLocalApp().SymbolLocator(),
				})
				if err != nil {
					return c.Send("Error initializing agent.")
				}
				defer ag.Close()

				ag.ResetSession()
				if err := ag.SaveSession(sessionName); err != nil {
					return c.Send("Failed to clear session.")
				}
				return c.Send("Context cleared for session: " + sessionBase)
			})

			b.Handle("/save", func(c tele.Context) error {
				sessionBase := activeSessions[c.Chat().ID]
				if sessionBase == "" {
					sessionBase = "default"
				}
				sessionName := fmt.Sprintf("tg-%d-%s", c.Chat().ID, sessionBase)

				ag, err := agent.New(projectRoot, providerCfg, agent.Config{
					SymbolLocator: GetLocalApp().SymbolLocator(),
				})
				if err != nil {
					return c.Send("Error initializing agent.")
				}
				defer ag.Close()

				if err := ag.SaveSession(sessionName); err != nil {
					return c.Send("Failed to save session: " + err.Error())
				}
				return c.Send("Session saved: " + sessionBase)
			})

			b.Handle("/compact", func(c tele.Context) error {
				sessionBase := activeSessions[c.Chat().ID]
				if sessionBase == "" {
					sessionBase = "default"
				}
				sessionName := fmt.Sprintf("tg-%d-%s", c.Chat().ID, sessionBase)

				ag, err := agent.New(projectRoot, providerCfg, agent.Config{
					SymbolLocator: GetLocalApp().SymbolLocator(),
				})
				if err != nil {
					return c.Send("Error initializing agent.")
				}
				defer ag.Close()

				if err := ag.LoadSession(sessionName); err != nil {
					return c.Send("Failed to load session.")
				}

				if err := ag.CompactContext(); err != nil {
					return c.Send("Failed to compact context: " + err.Error())
				}

				if err := ag.SaveSession(sessionName); err != nil {
					return c.Send("Failed to save compacted session.")
				}
				return c.Send("Session context compacted successfully.")
			})

			b.Handle(tele.OnText, func(c tele.Context) error {
				sessionBase := activeSessions[c.Chat().ID]
				if sessionBase == "" {
					sessionBase = "default"
				}
				sessionName := fmt.Sprintf("tg-%d-%s", c.Chat().ID, sessionBase)

				ag, err := agent.New(projectRoot, providerCfg, agent.Config{
					SymbolLocator: GetLocalApp().SymbolLocator(),
				})
				if err != nil {
					return c.Send("Error initializing agent.")
				}
				defer ag.Close()

				if err := ag.LoadSession(sessionName); err != nil {
					return c.Send("Error loading session.")
				}

				_ = c.Notify(tele.Typing)
				prompt := c.Text()

				if strings.HasPrefix(prompt, "!!") {
					cmdStr := strings.TrimSpace(strings.TrimPrefix(prompt, "!!"))
					c.Send("<i>Running local command...</i>", &tele.SendOptions{ParseMode: tele.ModeHTML})
					out, err := ag.RunBash(context.Background(), cmdStr, 60000, nil)
					if err != nil {
						return sendLongMessage(c, fmt.Sprintf("❌ Failed: %v\n%s", err, out))
					}
					return sendLongMessage(c, out)
				}

				if strings.HasPrefix(prompt, "!") {
					cmdStr := strings.TrimSpace(strings.TrimPrefix(prompt, "!"))
					out, err := ag.RunBash(context.Background(), cmdStr, 60000, nil)
					if err != nil {
						prompt += fmt.Sprintf("\nCommand !%s failed: %v\n%s", cmdStr, err, out)
					} else {
						prompt += fmt.Sprintf("\nOutput of !%s:\n%s", cmdStr, out)
					}
				}

				outChan := make(chan agent.AgentEvent)
				go ag.RunStream(prompt, outChan)

				statusMsg, _ := c.Bot().Send(c.Chat(), "<i>Thinking...</i>", &tele.SendOptions{ParseMode: tele.ModeHTML})
				var textChunk strings.Builder
				for ev := range outChan {
					if ev.Type == "text" {
						textChunk.WriteString(ev.Content)
					}
				}

				c.Bot().Delete(statusMsg)
				if err := ag.SaveSession(sessionName); err != nil {
					slog.Error("Failed to save session", "error", err)
				}

				return sendLongMessage(c, textChunk.String())
			})

			slog.Info("Telegram bot is running...")
			b.Start()
			return nil
		},
	}
	cmd.Flags().String("meili-url", os.Getenv("MEILI_URL"), "Meilisearch URL")
	cmd.Flags().String("meili-api-key", os.Getenv("MEILI_API_KEY"), "Meilisearch API key")
	cmd.Flags().StringP("project", "P", "", "Project root path (default is current directory)")
	cmd.Flags().String("allowed-users", "", "Comma separated list of allowed telegram user IDs or usernames")
	return cmd
}
