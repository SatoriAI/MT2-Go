# The CLAUDE.md budget

`CLAUDE.md` is loaded into the context window of every agent session, so its length is a cost paid on every piece of work rather than a matter of taste. It grows by accretion, one reasonable paragraph at a time, each defensible on its own, and no single commit looks like the problem. Only a hard number notices.

## Characters, not lines

The blueprint counts characters, and that is a measured correction it carries from an earlier project. A line budget there read 114 lines and passed for months on a file of 74,940 characters, because one of those lines was 20,370 characters long. A conventions file is written in paragraphs, and a paragraph is one line, so lines were not the quantity the check meant.

## 10,000, not the blueprint's 20,000

The blueprint allows 20,000 characters, roughly what 250 lines was meant to permit. This file held 6,845 when the budget was set on 2026-11-02, so 20,000 would have let it triple before anything objected. 10,000 leaves about half again as much room as the rules use today. It is meant to be raised, in the pull request that needs it and with the reason, when the rules genuinely grow; going over because case history crept in means the history belongs in this directory instead.

## Checked locally, not only in CI

The blueprint checks the budget only in the Repository workflow, so the first warning came after a push. Here it is part of `scripts/check_structure.sh`, which `make check` and therefore the pre-commit hook run, and which CI runs too. One check in one place, seen before the commit lands.
