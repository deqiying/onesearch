---
name: onesearch-deep-research
description: Use when an AI agent needs Onesearch Deep Research planning for complex, multi-step research without executing provider calls during planning.
---

# Onesearch Deep Research Skill

Use this skill for complex research questions that need decomposition, evidence planning, and gap checks.

Prefer this command first:

```powershell
onesearch deep "research question" --budget standard --format json
```

Workflow:

- `onesearch deep` is an offline planner. It does not call search, fetch, map, doctor, or providers.
- Read `intent_signals`, `decomposition`, `capability_plan`, `steps`, and `gap_check`.
- Execute the returned `steps[].command` with existing Onesearch CLI commands.
- Fetch key sources before claim-level statements; use `gap_check` to fetch missing evidence or downgrade unsupported claims.
- Do not force the task into a fixed topic recipe. The plan should follow the user's actual intent and risk level.
- Run `onesearch doctor --format json` separately when configuration or provider availability is uncertain.

Deep planning does not change the default route order or provider configuration.
