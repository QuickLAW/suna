Compress the following agent conversation history into a durable working summary for continuing the same task.

Preserve information needed to continue correctly:
- User goals, explicit requirements, constraints, and corrections.
- Current task status, open questions, blockers, and next steps.
- Relevant facts, decisions, entities, references, artifacts, inputs, outputs, and constraints that are still needed.
- Tool results only when they affect the current state, especially failures, discovered facts, generated artifacts, external responses, or completed actions.
- Important caveats about what was not done or could not be verified.

Discard information that is not useful for future steps:
- Verbatim long tool output, logs, raw data, transcripts, or document contents unless exact snippets are essential.
- Repetitive reasoning, transient exploration, and superseded hypotheses.
- Polite filler and formatting details.

Write a concise but specific summary. Prefer bullets. Do not invent facts. If the conversation language is mostly Chinese, write Chinese; otherwise use the conversation's primary language.

{{.Content}}
