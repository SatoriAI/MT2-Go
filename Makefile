# Every command this project has. CI invokes these targets rather than restating their
# commands, so the two cannot drift.
.PHONY: help hooks structure comment-blocks fmt-check lint test check db stop run account

# .env is optional; when present, every target sees its variables.
-include .env
export

help:
	@grep -E '^[a-z-]+:.*?## .*$$' $(MAKEFILE_LIST) | sed -E 's/:.*## /\t/' | expand -t24

hooks: ## Install the git hooks (once per clone)
	pre-commit install

structure: ## Assert the repository still has the shape the blueprint gave it
	./scripts/check_structure.sh

# A comment of eight lines or more is case history, and case history belongs in
# docs/decisions/. Standard library only, so it runs on any Python without this project's
# environment.
comment-blocks: ## Refuse a comment block that has started narrating
	python3 -m unittest discover --start-directory scripts --pattern 'test_*.py'
	python3 scripts/check_comment_blocks.py

fmt-check: ## Fail if any Go file is not gofmt-formatted
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then \
		echo "Not gofmt-formatted (run gofmt -w):"; echo "$$unformatted"; exit 1; fi

lint: ## go vet and golangci-lint
	go vet ./...
	go tool golangci-lint run

test: ## The test suite; it needs `make db` (go test -short ./... skips the PostgreSQL tests)
	go test -race ./...

# The definition of done, and the only one. A pre-commit hook runs it, so no commit lands
# red; never pass --no-verify. Cheapest first, so the fast answers come back before the
# slow ones are paid for.
check: structure comment-blocks fmt-check lint test ## Everything that decides whether this is green

db: ## Start PostgreSQL and create the test database
	docker compose up -d --wait db
	@docker compose exec -T db psql -U mt2go -tc "SELECT 1 FROM pg_database WHERE datname = 'mt2go_test'" | grep -q 1 \
		|| docker compose exec -T db createdb -U mt2go mt2go_test

stop: ## Stop PostgreSQL (the data stays in its volume)
	docker compose stop

run: ## Run the server on the ports in .env
	go run ./cmd/mt2go serve

account: ## Create an account: make account NAME=admin (asks for the password)
	@test -n "$(NAME)" || { echo "usage: make account NAME=<name>"; exit 1; }
	@read -rs -p "Password for $(NAME): " password; echo; \
		printf '%s\n' "$$password" | go run ./cmd/mt2go account create "$(NAME)"
