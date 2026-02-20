<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Core Workflow

See [rules/beads-workflow.md](rules/beads-workflow.md) for the complete workflow including:

- Issue tracking with bd CLI
- Token usage tracking
- Session completion checklist
- Git commit requirements

## Commit After Every Edit

After creating or editing any file, commit immediately. Do not accumulate uncommitted changes across multiple turns. Each round of edits gets its own commit before responding to the user. This applies to all file types: code, docs, rules, config.

## Code Implementation

When implementing code, follow [rules/code-prd-architecture-linking.md](rules/code-prd-architecture-linking.md):

- Link code to PRDs and architecture documents
- Include PRD references in commit messages
- Add package-level comments listing implemented PRDs

## Documentation

When writing documentation, follow [rules/documentation-standards.md](rules/documentation-standards.md) for style, formatting, and content quality.

For specific document types, see:

- [rules/prd-format.md](rules/prd-format.md) - Product Requirements Documents
- [rules/use-case-format.md](rules/use-case-format.md) - Use cases and tracer bullets
- [rules/vision-format.md](rules/vision-format.md) - Vision documents
- [rules/architecture-format.md](rules/architecture-format.md) - Architecture documents
- [rules/crumb-format.md](rules/crumb-format.md) - How to structure crumbs (docs vs code)
