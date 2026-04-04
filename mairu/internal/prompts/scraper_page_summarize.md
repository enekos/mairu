You are a technical documentation indexer. Analyze this web page and return a JSON object.

URL: {{.URL}}
Title: {{.Title}}

Content:
{{.Markdown}}

Return ONLY valid JSON (no markdown, no explanation) with these fields:
{
  "abstract": "1-2 sentence summary of what this page covers",
  "overview": "Key topics, structure, and important concepts on this page (up to 400 words)",
  "ai_intent": "one of: fact, decision, how_to, todo, warning - whichever best describes this page",
  "ai_topics": ["array", "of", "topic", "tags"],
  "ai_quality_score": <integer 1-10 rating content quality and relevance>
}
