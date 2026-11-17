# GitHub: the inputs that the workflows in .github/workflows need, and the
# settings of the repository itself (docs/guides/github-settings.md).

.PHONY: codeql-languages lint-pr-title github-settings

# The CodeQL languages that have source files, as the JSON matrix of codeql.yml.
codeql-languages:
	@tools/github/codeql-languages.sh

# A pull request is squash-merged with its title as the commit message, and
# release-please reads that message on main. CI passes the title in PR_TITLE:
#   PR_TITLE='feat: add orders' make lint-pr-title
lint-pr-title:
	@if [ -z "$${PR_TITLE:-}" ]; then echo "lint-pr-title: set PR_TITLE to the title of the pull request" >&2; exit 2; fi
	@printf '%s\n' "$$PR_TITLE" | tools/scripts/commit-msg.sh -

# Compare the settings of the repository on GitHub with the guide and print the
# differences. APPLY=1 changes them. REPO=<owner>/<name> picks another repository.
github-settings:
	@tools/github/apply-settings.sh $(if $(APPLY),--apply) $(REPO)
