package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"mairu/internal/prompts"
)

type CouncilRole struct {
	Name string
	Goal string
}

type CouncilConfig struct {
	Enabled bool
	Roles   []CouncilRole
}

func defaultCouncilRoles() []CouncilRole {
	return []CouncilRole{
		{
			Name: "App Developer",
			Goal: "Check that the feature change makes sense and follows practical product heuristics.",
		},
		{
			Name: "Developer Evangelist",
			Goal: "Review style, architecture, pitfalls, and potential performance issues.",
		},
		{
			Name: "Tests Evangelist",
			Goal: "Check that relevant tests are added or updated for changed behavior.",
		},
	}
}

func (c CouncilConfig) withDefaults() CouncilConfig {
	out := c
	if len(out.Roles) == 0 {
		out.Roles = defaultCouncilRoles()
	}
	return out
}

type councilFeedback struct {
	role     string
	feedback string
	err      error
}

func (a *Agent) runCouncil(outChan chan<- AgentEvent, task string) (string, error) {
	cfg := a.council.withDefaults()
	outChan <- AgentEvent{Type: "status", Content: "Council mode enabled: convening expert review"}

	resultsCh := make(chan councilFeedback, len(cfg.Roles))
	var wg sync.WaitGroup
	for _, role := range cfg.Roles {
		wg.Add(1)
		go func(r CouncilRole) {
			defer wg.Done()
			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("[Council][%s] reviewing task", r.Name)}
			feedback, err := a.generateCouncilFeedback(task, r)
			if err == nil {
				outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("[Council][%s] delivered review", r.Name)}
			}
			resultsCh <- councilFeedback{
				role:     r.Name,
				feedback: strings.TrimSpace(feedback),
				err:      err,
			}
		}(role)
	}
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	feedbackByRole := map[string]string{}
	var failures []string
	for result := range resultsCh {
		if result.err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", result.role, result.err))
			continue
		}
		if result.feedback == "" {
			failures = append(failures, fmt.Sprintf("%s: empty feedback", result.role))
			continue
		}
		feedbackByRole[result.role] = result.feedback
	}

	if len(feedbackByRole) == 0 {
		if len(failures) == 0 {
			return "", fmt.Errorf("no council feedback generated")
		}
		return "", fmt.Errorf("all council reviewers failed: %s", strings.Join(failures, "; "))
	}

	outChan <- AgentEvent{Type: "status", Content: "Council Product Lead: synthesizing expert reviews"}
	synthesized, err := a.generateProductLeadPlan(task, feedbackByRole)
	if err != nil {
		return "", fmt.Errorf("product lead synthesis failed: %w", err)
	}
	outChan <- AgentEvent{Type: "status", Content: "Council Product Lead: final guidance ready"}
	return buildCouncilExecutionPrompt(task, synthesized, feedbackByRole), nil
}

func (a *Agent) generateCouncilFeedback(task string, role CouncilRole) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	prompt, err := prompts.GetForProject("council_expert", struct {
		Role string
		Goal string
		Task string
	}{
		Role: role.Name,
		Goal: role.Goal,
		Task: task,
	}, a.root)
	if err != nil {
		return "", err
	}
	return a.llm.GenerateContent(ctx, a.llm.GetModelName(), prompt)
}

func (a *Agent) generateProductLeadPlan(task string, feedback map[string]string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	prompt, err := prompts.GetForProject("council_product_lead", struct {
		Task     string
		Feedback string
	}{
		Task:     task,
		Feedback: formatCouncilFeedback(feedback),
	}, a.root)
	if err != nil {
		return "", err
	}
	return a.llm.GenerateContent(ctx, a.llm.GetModelName(), prompt)
}

func formatCouncilFeedback(feedback map[string]string) string {
	roles := make([]string, 0, len(feedback))
	for role := range feedback {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	var b strings.Builder
	for _, role := range roles {
		b.WriteString("## ")
		b.WriteString(role)
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(feedback[role]))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

func buildCouncilExecutionPrompt(task, synthesis string, feedback map[string]string) string {
	var b strings.Builder
	b.WriteString("You are executing with guidance from a council of reviewers.\n\n")
	b.WriteString("## Original Task\n")
	b.WriteString(strings.TrimSpace(task))
	b.WriteString("\n\n")
	b.WriteString("## Product Lead Guidance\n")
	b.WriteString(strings.TrimSpace(synthesis))
	b.WriteString("\n\n")
	b.WriteString("## Expert Reviews\n")
	b.WriteString(formatCouncilFeedback(feedback))
	b.WriteString("\n\n")
	b.WriteString("Follow this guidance pragmatically while completing the task. If guidance conflicts with repository reality, explain the conflict and choose the safest implementation.")
	return b.String()
}
