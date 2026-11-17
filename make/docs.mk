# Markdown: structure, links, English prose (Vale), and Japanese prose (textlint).
# English lives in foo.md and its Japanese translation in foo.ja.md.

FMT_TARGETS += fmt-md
CHECK_TARGETS += lint-md lint-prose-en lint-prose-ja lint-links
CHECK_ALL_TARGETS += lint-links-remote

MD_EN := "*.md" ":(exclude)*.ja.md"
MD_JA := "*.ja.md"

fmt-md:
	@$(FILES) "*.md" | xargs -r markdownlint-cli2 --fix > /dev/null 2>&1 || true

lint-md:
	@$(TASK) $@ '$(FILES) "*.md" | xargs -r markdownlint-cli2 2>&1 | { grep -Ev "^(markdownlint-cli2 v|Finding: |Linting: |Summary: )" || true; }'

lint-prose-en:
	@$(TASK) $@ '$(FILES) $(MD_EN) | xargs -r vale --output=line --no-wrap'

lint-prose-ja:
	@$(TASK) $@ '$(FILES) $(MD_JA) | xargs -r textlint --format unix | sed "s|$(CURDIR)/||"'

# Links between files in the repository, including heading anchors. No network.
lint-links:
	@$(TASK) $@ '$(FILES) "*.md" | xargs -r lychee --offline --include-fragments --no-progress --format compact'

# Every link, including the ones that leave the repository.
lint-links-remote:
	@$(TASK) $@ '$(FILES) "*.md" | xargs -r lychee --include-fragments --no-progress --format compact'
