---
name: commit
description: Use when the user asks to create, generate, or write a git commit message, or mentions /commit, conventional commits, or commit message. Provides guidelines for drafting Conventional Commits-style messages from staged/unstaged changes.
---

# Commit — Conventional Commit Writer

Generate a clean, standard Git commit message following the [Conventional Commits](https://www.conventionalcommits.org/) specification, based on the current repository changes (and any extra context the user provides).

## Argument Parsing

The user's request may contain:

- An optional language flag: `--lang=<code>` or `--lang <code>` (e.g. `--lang=zh`, `--lang=en`, `--lang=ja`)
  - If absent, default to **English (`en`)**.
- Everything after the flag is the free-form description of the change.

## Workflow

1. Parse the user's request to extract `lang` and `description`.
2. Run these in parallel:
   - `git status` (no `-uall`)
   - `git diff` (unstaged)
   - `git diff --cached` (staged)
   - `git log -n 5 --oneline`
3. Decide what is actually changing. If `description` is non-empty, reconcile it with the diff.
4. Draft the message in Conventional Commits format, **written in the chosen language**. The Conventional Commits prefix (`type(scope):`) stays in English regardless of `lang`.
5. Show the drafted message to the user. **Do not commit automatically.** Wait for the user to confirm.
6. Only when the user explicitly confirms, stage relevant files and run `git commit` using a HEREDOC.

## Output Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

- `type` and `subject` are **required**.
- `scope`, `body`, `footer` only when they add meaningful information.
- Output language follows `lang` (default `en`). The `type(scope):` prefix is **always** in English.

## Edge Cases

- **Vague input** with no diff signal: ask one clarifying question.
- **Multiple unrelated changes**: suggest splitting.
- **No changes detected**: tell the user and stop.

## Commit Execution (only after user confirmation)

Use a HEREDOC to preserve formatting:

```bash
git commit -m "$(cat <<'EOF'
<type>(<scope>): <subject>

<body>

<footer>
EOF
)"
```

Never use `--no-verify`, never amend, never force-push.
