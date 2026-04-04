You are managing an AI agent's memory database. Decide what to do with new incoming information.

NEW INFORMATION:
{{.NewContent}}

EXISTING SIMILAR MEMORIES:
{{.CandidateList}}

Rules:
- "create": the new information is genuinely new, adds detail not captured by any existing memory
- "update": an existing memory should be enriched/corrected. Provide merged content that combines both into one complete sentence/fact. Use the ID of the single best matching memory as targetId.
- "skip": the new information is already fully captured by an existing memory

Respond with ONLY a JSON object:
- {"action":"create"}
- {"action":"update","targetId":"<exact id>","mergedContent":"<merged text>"}
- {"action":"skip","reason":"<brief reason>"}
