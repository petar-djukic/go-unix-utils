<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# Beads (bd) Issue Tracking and Session Completion Workflow

**MIGRATION NOTE**: This project is migrating to the cupboard CLI for issue tracking. See `.claude/rules/cupboard-workflow.md` for the cupboard workflow. The beads (bd) commands documented below will be replaced by cupboard commands (e.g., `cupboard ready`, `cupboard show`, `cupboard close`). For new work, refer to cupboard-workflow.md.

## Working Offline

**We work offline.** There is no access to the remote repository. **Local git commit works** and is required; **do not run `git push`** (or `git pull`). The user will sync with the remote when they have network access.

## Do Not Edit Beads Files Directly

**Never change files under `.beads/` by hand.** Do not edit `.beads/issues.jsonl` or any other file in `.beads/` with an editor or script. All issue creation, updates, comments, and status changes must go through the **bd** CLI (e.g. `bd update`, `bd comments add`, `bd close`, `bd sync`). Commits may include `.beads/` changes produced by `bd`; the agent must not modify those files directly.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd comments add <id> "tokens: <count>"  # Log token usage
bd close <id>         # Close work
bd sync               # Sync with git
```

## Token Tracking

**Track token usage for every issue:**

1. **At start of issue** - Note current token count from context
2. **When closing issue** - Calculate tokens used and log it:
   ```bash
   bd comments add <id> "tokens: <count>"
   bd close <id>
   ```

Example:
```bash
# Started with 1000000 tokens, now at 965744
# Used: 34256 tokens
bd comments add atlas-123 "tokens: 34256"
bd close atlas-123
```

## LOC and Documentation Tracking

**Track lines of code and documentation changes per issue:**

1. **At start of issue** - Run `mage stats` and note the baseline:
   ```bash
   mage stats
   # Save: LOC_PROD=441, LOC_TEST=0, DOC_WORDS=21032
   ```

2. **When closing issue** - Run the command again and calculate the delta:
   ```bash
   mage stats
   # New: LOC_PROD=520, LOC_TEST=45, DOC_WORDS=21900
   # Delta: +79 LOC (prod), +45 LOC (test), +868 words (docs)
   ```

3. **Include full stats in commit message** - Add the Stats block with totals and deltas:

   ```text
   Add feature X (issue-id)

   - Description of changes

   Stats:
     Lines of code (Go, production): 520 (+79)
     Lines of code (Go, tests):      45 (+45)
     Words (documentation):          21900 (+868)
   ```

   **Do NOT use a condensed format** like `Delta: +79 LOC (prod)...`. Always use the full Stats block.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until changes are committed locally.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status and log tokens**:
   - Calculate tokens used this session
   - Add comment with token count: `bd comments add <id> "tokens: <count>"`
   - Close finished work: `bd close <id>`
   - Update in-progress items
4. **COMMIT CHANGES** - This is MANDATORY:
   ```bash
   bd sync
   git add -A
   git commit -m "descriptive message"
   git status  # Verify all changes committed
   ```
   (Do not run `git push`; we have no remote access. Commit works locally.)
5. **Clean up** - Clear stashes; skip remote operations (we are offline).
6. **Verify** - All changes committed locally.
7. **Hand off** - Provide context for next session; inform user that changes are committed locally and they can push when they have network access. **When summarizing changes in code or markdown**, run `mage stats` and include its output (Go production/test LOC, doc words) in the summary.

**CRITICAL RULES:**
- Work is NOT complete until changes are committed locally
- NEVER leave uncommitted changes - commit everything
- **After creating or editing any files** (docs, code, use cases, rules, config), run `git add -A` and `git commit` with a descriptive message **before ending your turn**. Do not hand off with uncommitted changes.
- **We work offline** - Do not push; local commit is required and works. The user will push when they have network access.
