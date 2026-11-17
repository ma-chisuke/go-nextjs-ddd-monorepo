# GitHub: the inputs that the workflows in .github/workflows need, and the
# settings of the repository itself (docs/guides/github-settings.md).

.PHONY: lint-pr-title

# A pull request is squash-merged with its title as the commit message, and
# release-please reads that message on main. CI passes the title in PR_TITLE:
#   PR_TITLE='feat: add orders' make lint-pr-title
lint-pr-title:
	@if [ -z "$${PR_TITLE:-}" ]; then echo "lint-pr-title: set PR_TITLE to the title of the pull request" >&2; exit 2; fi
	@printf '%s\n' "$$PR_TITLE" | committed --commit-file - \
		|| { echo "lint-pr-title: write the title as a Conventional Commit, for example 'feat: add orders'" >&2; exit 1; }
