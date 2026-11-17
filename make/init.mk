# Initialize a repository created from the template: replace the Go module path
# and the project name, and reset the changelog. The command is tools/init.

.PHONY: init
init: ## Make a copy of the template your own: MODULE=github.com/<owner>/<repo> [NAME=<name>] [DRY_RUN=1]
	@cd tools && go build -o "$(CURDIR)/.cache/bin/init" ./init
	@.cache/bin/init -C "$(CURDIR)" -module "$(MODULE)" -name "$(NAME)" $(if $(filter 1,$(DRY_RUN)),-dry-run)
