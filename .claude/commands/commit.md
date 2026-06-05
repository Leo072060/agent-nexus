---
description: Generate a Conventional Commits-style commit message from the current staged/unstaged changes or the user's description. Supports --lang=<code> to control output language (default: en).
argument-hint: [--lang=en|zh|...] [optional description of changes]
---

# /commit — Conventional Commit Writer

Generate a clean, standard Git commit message following the [Conventional Commits](https://www.conventionalcommits.org/) specification, based on the current repository changes (and any extra context the user provides in `$ARGUMENTS`).

## Argument Parsing

`$ARGUMENTS` may contain:

- An optional language flag at the start: `--lang=<code>` or `--lang <code>` (e.g. `--lang=zh`, `--lang=en`, `--lang=ja`).
  - If absent, default to **English (`en`)**.
  - If the user wrote the description in a non-English language but did **not** pass `--lang`, still default to English (matches the project's commit style).
- Everything after the flag is the free-form description of the change.

Examples of how to interpret arguments:

| `$ARGUMENTS`                                | lang | description                              |
| ------------------------------------------- | ---- | ---------------------------------------- |
| *(empty)*                                   | en   | *(none — infer from diff)*               |
| `fixed null pointer in config loader`       | en   | `fixed null pointer in config loader`    |
| `--lang=zh 修复了配置加载时的空指针`         | zh   | `修复了配置加载时的空指针`                 |
| `--lang zh`                                 | zh   | *(none — infer from diff, output Chinese)* |

## Workflow

1. Parse `$ARGUMENTS` per the rules above to extract `lang` and `description`.
2. Run these in parallel to inspect the working tree:
   - `git status` (no `-uall`)
   - `git diff` (unstaged)
   - `git diff --cached` (staged)
   - `git log -n 5 --oneline` (match repo's commit style)
3. Decide what is actually changing. If `description` is non-empty, treat it as the user's intent and reconcile it with the diff.
4. Draft the message in the format below, **written in the chosen language**. The Conventional Commits prefix (`type(scope):`) stays in English regardless of `lang` — only the human-readable parts (subject text, body, footer prose) are translated.
5. Show the drafted message to the user. **Do not commit automatically.** Wait for the user to confirm or ask for edits.
6. Only when the user explicitly confirms (e.g. "提交"/"commit it"/"yes"), stage the relevant files and run `git commit` using a HEREDOC.

## Output Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

- `type` and `subject` are **required**.
- `scope`, `body`, `footer` only when they add meaningful information.
- Output language follows `lang` (default `en`). The `type(scope):` prefix is **always** in English; only subject text, body, and footer prose are translated.

## Examples

### Minimal

User context: "fixed a null pointer crash when the config file is missing"

```
fix(config): handle missing config file gracefully
```

### With body

User context: "rewrote the tokenizer to use a state machine instead of regex, much faster now"

```
refactor(tokenizer): replace regex parsing with state machine

Previous implementation used a chain of regex patterns which
degraded to O(n²) on deeply nested input. The new state machine
runs in linear time and simplifies error recovery.
```

### Breaking change

User context: "removed the --output flag, users should use --out instead"

```
feat(cli): rename --output flag to --out

BREAKING CHANGE: --output is no longer supported. Replace all
usages with --out.
```

### Issue reference

User context: "added pagination to the search results API, closes issue 88"

```
feat(api): add pagination to search results endpoint

Closes #88
```

### Chinese output (`--lang=zh`)

User context: `--lang=zh 重构了 tokenizer 用状态机替代正则，性能更好`

```
refactor(tokenizer): 用状态机替代正则解析

旧实现使用一连串正则模式，在深度嵌套输入下退化为 O(n²)。
新的状态机以线性时间运行，并简化了错误恢复逻辑。
```

## Edge Cases

- **Vague input** ("updated some stuff" with no diff signal): ask **one** clarifying question — *what* changed and *why*.
- **Multiple unrelated changes**: note that mixing unrelated changes in one commit is discouraged, draft the best single message for the dominant change, and suggest splitting.
- **Language mismatch**: if `--lang` was not passed but the user's description is in another language, still output English (the safe default). Mention once that they can pass `--lang=<code>` to change it.
- **Diff already speaks for itself**: infer intent directly; do not ask unless genuinely ambiguous. Keep the message concise.
- **No changes detected**: tell the user there is nothing to commit and stop.

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

Never use `--no-verify`, never amend, never force-push. Do not add `Co-Authored-By` lines unless the user asks.

---

User-provided context (may be empty): $ARGUMENTS
