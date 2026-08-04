---
name: onesearch-deep-research
description: Use when an AI agent needs Onesearch Deep Research planning for complex, multi-step research without executing provider calls during planning.
---

# Onesearch Deep Research

Use this skill for complex questions that need decomposition, evidence planning, and an explicit gap check. `onesearch deep` is an offline planner: it does not call search, fetch, map, `doctor`, or any provider.

## Discover the Contract

```text
onesearch schema deep --format json
```

## Plan and Execute

```text
onesearch deep "research question" --budget standard --format json
```

1. Read `intent_signals`, `decomposition`, `capability_plan`, `preflight`, `steps`, and `gap_check`.
2. Substitute any runtime placeholders with validated values.
3. Run `onesearch status --format json` before executing planned dynamic steps.
4. Execute each `preflight[].command_argv` or `steps[].command_argv` as an argv token array.
5. Fetch key sources before claim-level conclusions, then use `gap_check` to acquire missing evidence or downgrade unsupported claims.

The compatible `command` field is display-only. Never split it on spaces, reinterpret quoting, or execute it in place of `command_argv`.

If a planned provider is unavailable, replace it only with a semantically equivalent available workflow/provider or report the gap. Do not force every task into a fixed topic recipe.

## Output and Recovery

Use compact JSON for plan execution. Use `--format content` only when a human-readable plan body is sufficient, and `--verbose` when full structured details are needed.

On `parameter_error`, inspect only `schema deep`. Configuration/provider errors can arise later when executing planned commands; recover according to each command's own skill and typed error.
