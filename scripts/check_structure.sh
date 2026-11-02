#!/usr/bin/env bash
# Assert the repository still has the shape the blueprint gave it.
#
# This is the difference between a convention and a rule: a structure nothing checks is one
# that drifts a hurried commit at a time. Run by `make check`.
set -euo pipefail

fail=0
note() { echo "::error::$1"; fail=1; }

# Directories that are not this repository's source: dependencies, caches, build output.
PRUNE=(-name .git -o -name .venv -o -name node_modules -o -name dist -o -name .claude
       -o -name __pycache__ -o -name .mypy_cache -o -name .ruff_cache -o -name .pytest_cache)
# -print0 throughout: a file name with a space would otherwise split into two paths
# that match nothing, and the scan below would pass by finding nothing to read.
sources() { find . \( "${PRUNE[@]}" \) -prune -o -type f -print0; }

# The files every project has.
for file in README.md CLAUDE.md AGENTS.md Makefile .gitignore .env.example \
            .pre-commit-config.yaml docs/decisions/README.md \
            .github/dependabot.yml .github/workflows/repository.yml \
            scripts/check_comment_blocks.py; do
  [ -f "$file" ] || note "$file is missing."
done

# The gate must exist and must be what CI calls.
grep -q '^check:' Makefile || note "Makefile has no 'check' target; it is the definition of done."
grep -q '^check:.*structure' Makefile || note "'make check' no longer runs the structure check."

# A Python service carries its own pins. Checked only where one exists, so the base
# blueprint on its own still passes.
if [ -f backend/pyproject.toml ]; then
  for file in backend/.python-version backend/uv.lock backend/Dockerfile .dockerignore \
              .github/workflows/backend.yml .github/actions/setup-python/action.yml; do
    [ -f "$file" ] || note "$file is missing, and backend/pyproject.toml says this is a Python service."
  done
fi

# A project deploying on Railway carries the file, its pinned SDK and the workflow that
# applies it. `npm ci` in that workflow needs the lockfile to be committed.
if [ -f .railway/railway.ts ]; then
  for file in .railway/package.json .railway/package-lock.json .github/workflows/railway.yml; do
    [ -f "$file" ] || note "$file is missing, and .railway/railway.ts says this project deploys on Railway."
  done
fi

# CLAUDE.md is loaded into every session, so its length is a cost paid on every task and
# it grows a defensible paragraph at a time. Over budget: move reasoning to docs/decisions/,
# or raise the budget here and say why. See docs/decisions/claude-md-budget.md.
claude_budget=10000
claude_size=$(wc -c < CLAUDE.md | tr -d ' ')
if [ "$claude_size" -gt "$claude_budget" ]; then
  note "CLAUDE.md is $claude_size characters, over its $claude_budget-character budget."
fi

# Placeholders left behind mean the setup was never finished. Named explicitly: a bare
# `{{` also matches a GitHub Actions expression, a JSX style object and a template
# language, all of which are legitimate source.
left=$(sources | xargs -0 -r grep -In -E '\{\{ (project|Project|package|purpose|stack|owner|app_port|api_port|db_port) \}\}' 2>/dev/null || true)
if [ -n "$left" ]; then
  note "Blueprint placeholders are still in the tree:"
  echo "$left" | head -20
fi

# The blueprint's own prompts, left in a file that is supposed to describe this project.
# The blueprint's own prompts are `TODO:` at the start of a line or `| TODO |` in a table,
# so a sentence about TODO comments in prose does not trip this.
for file in CLAUDE.md README.md docs/decisions/README.md; do
  [ -f "$file" ] && grep -qE '^TODO:|\| TODO \|' "$file" && note "$file still carries the blueprint's TODO prompts."
done

# Secrets are configured, never committed.
[ -f .env ] && git ls-files --error-unmatch .env > /dev/null 2>&1 && note ".env is tracked by git."

# Every action is pinned to a commit SHA, so a moved tag cannot change what CI runs.
if ls .github/workflows/*.yml > /dev/null 2>&1; then
  unpinned=$(grep -rhE '^\s*-?\s*uses: ' .github | grep -vE '@[0-9a-f]{40}' | grep -vE 'uses: \./' || true)
  if [ -n "$unpinned" ]; then
    note "These actions are not pinned to a commit SHA:"
    echo "$unpinned"
  fi
fi

# Every variable the code reads is documented where people look for it. Names only: this
# cannot see a variable read through a computed string, and does not try.
if [ -f .env.example ]; then
  # `name(` or `name[`, then the opening quote, then the variable. The quote is matched as
  # a run of non-word characters so the pattern needs no nested shell quoting.
  read_names=$(sources | grep -z '\.py$' | xargs -0 -r grep -hoE \
                 '(getenv|environ\.get|environ|\bconfig)[([][^A-Za-z0-9_]*[A-Z][A-Z0-9_]+' 2>/dev/null \
               | grep -oE '[A-Z][A-Z0-9_]+$' | sort -u || true)
  for name in $read_names; do
    grep -qE "^#? *${name}=" .env.example || note "$name is read by the code but not documented in .env.example."
  done
fi

[ "$fail" -eq 0 ] && echo "Structure OK."
exit "$fail"
