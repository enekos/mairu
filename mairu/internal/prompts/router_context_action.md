You are managing a hierarchical context database for a software project. Decide what to do with a new context node.

NEW NODE:
URI: {{.URI}}
NAME: {{.Name}}
ABSTRACT: {{.Abstract}}

EXISTING SIMILAR NODES:
{{.CandidateList}}

Rules:
- "create": the new node covers genuinely new territory
- "update": an existing node should have its abstract enriched. Provide merged abstract as mergedContent. Use the existing node's URI as targetId.
- "skip": the new node is already fully covered by an existing node

Respond with ONLY a JSON object:
- {"action":"create"}
- {"action":"update","targetId":"<exact uri>","mergedContent":"<merged abstract>"}
- {"action":"skip","reason":"<brief reason>"}
