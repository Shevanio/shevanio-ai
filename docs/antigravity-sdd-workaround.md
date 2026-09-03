# Antigravity SDD

Antigravity runs Shevanio AI's SDD phases inline because it does not expose custom phase subagents. Mission Control may still delegate browser or terminal work to its built-in agents, but the SDD orchestrator remains in one conversation.

## Current behavior

| Area | Contract |
|---|---|
| Instructions | Shevanio AI appends its managed prompt to `~/.gemini/GEMINI.md`. |
| Skills | Native skills live under `~/.gemini/antigravity/skills/`. |
| MCP | Servers are merged into `~/.gemini/antigravity/mcp_config.json`. |
| SDD artifacts | The selected Engram, OpenSpec, or hybrid store preserves phase state. Do not create a parallel `.sdd/` state convention. |
| Model routing | Antigravity is single-mode; the active model handles every phase. |

## Recommended flow

1. Install or sync the Antigravity integration with Shevanio AI.
2. Keep small work direct. Use SDD only after an explicit request or an accepted proposal.
3. Review persisted proposal, spec, design, task, and verification artifacts at phase boundaries instead of relying only on chat history.
4. Start a fresh session if the inline context becomes unreliable; recover from the persisted artifacts rather than reconstructing state from memory.

This is a capability boundary, not an emulated multi-agent system. See [Supported Agents](agents.md) for the current matrix and [Intended Usage](intended-usage.md) for routing guidance.
