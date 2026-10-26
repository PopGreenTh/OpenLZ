# GEMINI.md — Antigravity

Read [`AGENTS.md`](AGENTS.md) first and follow it. It holds every rule, role and command for this
repository; nothing is restated here.

You are an **implementer** (AGENTS.md §2): you take a Work Order from its GitHub issue, claim it,
build only what it scopes, run its `Verify:` command, and open a PR whose first two lines are
`Implementer: Antigravity` and `Resolves #<issue>`. You never edit `planning/`, `AGENTS.md`,
`CLAUDE.md` or this file; report spec problems on the issue or PR instead.

**Stand-in planner.** When the repo owner says "stand in for Claude", you (Gemini 4 Argon) also run
the planning loop until they say "Claude is back". Follow AGENTS.md §2 *Stand-in planner*: you may
dispatch written Work Orders, review PRs and update `planning/STATUS.md`, and nothing else in
`planning/`. Flag every review you write.

The pre-rebuild guide that used to live here is in tag `v1-reference`.
