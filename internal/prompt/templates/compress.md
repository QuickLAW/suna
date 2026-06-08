Compress the following agent conversation history into a continuation state for continuing the same task.

The output is not a transcript summary. It is the working state the agent needs in order to continue without restarting, re-asking settled questions, or repeating rejected directions.

Always preserve both conversation state and execution state when present:
- Conversation state: user goal, explicit requirements, preferences, corrections, accepted decisions, rejected or superseded directions, current discussion point, open questions, and next steps.
- Execution state: tool/action facts, discovered facts, state changes, artifacts, failures, verification results, important references, and the current execution point.

Prioritize the user's explicit instructions, corrections, preferences, and decisions over the assistant's plans, guesses, or tool execution details. If the assistant proposed an approach that the user rejected or asked to simplify, record it only as rejected and preserve the user's accepted direction.

For tool use, convert tool calls and tool results into concise action facts. Keep only facts that affect the current task state: what was done, what changed, what was discovered, what failed, what was created, and what was verified. Do not summarize the conversation as a tool execution log. Do not include long raw logs, raw data, raw file contents, or verbose command output unless an exact snippet is essential.

Write in the conversation's primary language. Be concise but specific. Prefer bullets. Do not invent facts.

Use this exact structure:

# Continuation State

## User goal
- ...

## User constraints / preferences
- ...

## Decisions
- ...

## Rejected directions
- ...

## Current state / recent progress
- ...

## Tool/action facts
- ...

## Next steps
- ...

Conversation history to compress:

{{.Content}}
