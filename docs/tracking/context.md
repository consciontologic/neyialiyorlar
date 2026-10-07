# 🧠 docs/tracking/context.md — shared project context pack

> **This file is the single place for project-specific overrides.**
> All vendor entry points (`CLAUDE.md`, `GEMINI.md`, `CONVENTIONS.md`,
> `.github/copilot-instructions.md`, etc.) are invited to reference this file
> so context stays in sync without editing every vendor file.

---

## Project identity

<!-- Fill in after scaffolding -->
- **Name**: (your project name)
- **One-liner**: (what does this project do in ≤ 15 words)
- **Primary language**: (python / node / go / rust / …)
- **Repo URL**: (https://github.com/…)

## Key paths

| Concern | Path |
|---|---|
| Master rulebook | `AGENTS.md` |
| Project plan | `docs/planning/ROADMAP.md` |
| Tracking log | `docs/tracking/tracking.csv` |
| Skills library | `.agents/skills/` |
| Ops scripts | `xops/` |

## Active context (update as the project evolves)

<!-- What is the team / agent working on right now?
     One short paragraph is enough. Agents read this to orient fast. -->

_Not yet set. Fill in after first sprint._

## Project-specific conventions

<!-- Any rules that apply *only* to this project and are NOT already in AGENTS.md.
     E.g.: "All public APIs must have OpenAPI annotations."
           "Use pydantic v2 models everywhere — no plain dicts."
           "Commit messages must reference a JIRA ticket: PROJ-123." -->

_None yet._

## Out-of-scope / do not touch

<!-- List directories, files, or systems agents should treat as read-only.
     E.g.: "Never edit vendor/ — it is a git subtree." -->

_None yet._

## External service dependencies

<!-- List services this project calls, with the env var that holds each key.
     Do NOT include actual keys here — only the var names. -->

| Service | Env var | Notes |
|---|---|---|
| (example) Stripe | `STRIPE_SECRET_KEY` | Sandbox only in dev |

## Agent quick-reference

```bash
make help           # all targets
make doctor         # sanity-check framework install
make git.dry        # preview pending commits (read-only)
make git            # commit + push (human runs this)
make track.add ACTION=note SUMMARY="..."   # append tracking row
make skills.find TAG=<tag>                 # search skill library
make skills.status                         # skill index with line counts
```
