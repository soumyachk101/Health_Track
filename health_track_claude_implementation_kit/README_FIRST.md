# Health_Track — Claude Code Implementation Kit

This kit is for the existing repository:

`https://github.com/soumyachk101/Health_Track`

## Setup

1. Clone or open the existing repository.
2. Create a dedicated branch:
   ```bash
   git switch -c hardening/scalable-architecture
   ```
3. Extract this ZIP into the repository root. Review the file conflicts before allowing any overwrite.
4. Open the repository root in Claude Code.
5. Read `CLAUDE.md` and `docs/IMPLEMENTATION_MASTER_PROMPT.md`.
6. Paste the complete master prompt into Claude Code.
7. Ask Claude to perform Phase 0 (audit and plan) first. Review the generated audit, architecture decisions, risks, and task breakdown.
8. Execute one implementation phase at a time. After each phase, inspect the diff and require the tests listed in the quality gates.

## Important operating rules

- The kit is a specification, not authorization to replace the application wholesale.
- Claude must inspect the current checkout and trust the current source over this kit's repository snapshot.
- Preserve working features and public API compatibility unless a migration plan is documented and tested.
- Never run destructive database commands against production.
- Never print, commit, or expose secrets, access tokens, patient information, or production environment values.
- Do not claim that tests, deployments, security scans, or load tests passed unless they were actually run and their output was inspected.
- If a requirement cannot be implemented safely without a product decision, document the blocker and provide options instead of silently inventing behavior.
