# Contributing to MT2 Go

Keep each change focused, reviewable and green. `make help` lists the routine commands; `make check` must pass before you push, and the pre-commit hook runs it for you once you have run `make hooks`.

## Where code may come from

This repository is MIT-licensed and holds only code written for it.

- **Never copy code** from [open-mt2](https://github.com/willianmarquess/open-mt2) (GPL), from the game client, or from any other server. Read them for facts about the protocol and write the code yourself.
- **Never use, read or reference leaked original server code**, not even to check a detail.
- Facts go into `docs/mechanics/` in your own words, with where you learned them.

A change that breaks these rules is reverted, however useful it is.

## Getting started

```bash
cp .env.example .env
make db                  # PostgreSQL on localhost:15432, plus the test database
make account NAME=admin  # the local test account is admin / admin
make run                 # auth on 127.0.0.1:11002, game on 127.0.0.1:13001
```

The end-to-end tests need `make db`; they use the test database and never touch the development one. open-mt2 uses the same game ports, so run one server at a time.

## Mechanics

Every game mechanic has a page in `docs/mechanics/`: what the client does, the exact bytes, what the server does, how we know and which test proves it. A change to a mechanic updates its page in the same pull request. [`docs/mechanics/README.md`](docs/mechanics/README.md) has the template.

## Commits

- One logical change per commit, and every commit green.
- An imperative, sentence-case subject with no full stop, ideally 50 and at most 70 characters, naming the outcome rather than the files or mechanism. No Conventional Commits prefix.
- The body holds what the diff cannot show: the problem, the key decision, the alternatives rejected and what was verified. A trivial change may have a subject only.
- Check `git status` and the staged diff before committing, stage deliberately, and never bypass the hooks with `--no-verify`.
- Do not rewrite a pushed shared branch. Push follow-up commits during review.

## Pull requests

- One purpose per pull request, and about 400 changed lines of handwritten code at most. Generated files, lockfiles, fixtures and mechanical moves do not count.
- Title it by its concrete outcome. Open with a short summary of what is now true, why it matters and the key design decision.
- Group the changes by behaviour or package, not by file or commit.
- Report evidence as **Verified**, **Tested**, **Measured** or **Not checked**, and name the blind spots, for example "not tried with the real client".
- Point reviewers at the risks: what a hostile client could do, data and migration concerns, compatibility with the client, and what is deliberately left out.
- Record tangential findings as follow-ups instead of widening the change. Answer each review comment with accept, reject or defer, and a reason.
- Do not merge with red or pending checks, unresolved blocking comments or conflicts.

## Documentation

- `README.md` is the front door. How mechanics work goes in `docs/mechanics/`, and the reasoning behind decisions in `docs/decisions/`.
- Update the document that owns a behaviour in the same change as the behaviour. Delete guidance that is no longer true; version control keeps its history.
- Add or remove variables in `.env.example` in the same change as the code that reads them.
