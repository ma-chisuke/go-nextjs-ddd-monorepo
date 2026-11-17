---
status: accepted
date: 2026-11-17
---

# TPL-0021: Write documents in English, with a Japanese translation next to each

## Context and Problem Statement

This template is public, and most of the people who copy it read English.
Its owner reviews every document, and reviews in Japanese.
A mistake that review misses reaches every copy of the template.

Documents in two languages bring a risk of their own: two texts that disagree, with no way to tell which one is right or which one is out of date.

Which language is the source, where does the other language live, and how does a reader know that the two texts still agree?

## Decision Drivers

- The audience of a public template must be able to read every document.
- The owner must be able to review every document in Japanese.
- When the two texts differ, one of them must be the authority.
- A check, not a reader, must find a translation that is out of date.
- Code and instructions for tools must have one language, so that nothing there needs a translation.

## Considered Options

- English as the source, with a Japanese translation next to each document
- English only
- Japanese only
- Japanese as the source, with an English translation next to each document
- A separate tree for the translations (`docs/ja/`)
- Machine translation on demand, with nothing committed

## Decision Outcome

Chosen option: "English as the source, with a Japanese translation next to each document", because the public reads the source rather than a translation while the owner reviews in Japanese, the documents share the language of the code, and a check can tell when the two texts drift apart.

The rules are those of the [translation guide](../../guides/translation.md).

- `foo.md` is English and is the source. `foo.ja.md` is its Japanese translation and sits in the same directory.
- Both files change in the same commit.
- The last line of a translation records the SHA-256 hash of the English file that it translates. `make i18n-stamp` writes that line.
- Code, comments, commit messages, and the descriptions in OpenAPI documents are in English only.
- Files under `.claude/` and `.github/` are instructions for tools, and are in English only. Third-party files keep the language in which they were published, such as the Japanese writing norm.
- Vale checks English prose against the Google developer documentation style, and textlint checks Japanese prose. The Japanese writing norm is kept in the repository (`.claude/skills/japanese-tech-writing/`), and translations follow it.

### Consequences

- Good, because a reader of either language reads the whole document, not a summary.
- Good, because the English text decides when the two texts disagree.
- Good, because the two files sit side by side: relative links work the same in both, and a change to both shows in one diff.
- Good, because tools and agents read instructions in English only, so no instruction has two versions that can disagree.
- Bad, because every change to a document is written twice, and its review takes longer.
- Bad, because a contributor who does not write Japanese cannot update a translation alone, and needs the skill `sync-translation` or the help of a maintainer. `make i18n-stamp` would make `make check` pass, but the translation would stay out of date.
- Bad, because the hash proves only that someone ran `make i18n-stamp` after the source changed, not that the translation was updated or that the meaning matches. The subagent `translation-reviewer` compares the meaning, and it can miss a difference.
- Bad, because the owner reviews the translation rather than the source, so a mistake that exists only in the English text can pass review.

### Confirmation

- `make check` runs the target `lint-i18n`, which fails when a document has no translation, or when the hash in a translation differs from the hash of the English file.
- `make check` runs Vale on English Markdown (`lint-prose-en`) and textlint on Japanese Markdown (`lint-prose-ja`).
- The subagent `translation-reviewer` compares the meaning of a translation with its source, and review covers what it misses.
- Review confirms that code, comments, commit messages, and OpenAPI descriptions are in English. No check enforces the language.

## Pros and Cons of the Options

### English as the source, with a Japanese translation next to each document

- Good, because both readers get the whole text.
- Good, because it is clear which text is the authority.
- Bad, because the work of writing doubles.

### English only

- Good, because there is one text and nothing to keep in step.
- Bad, because the owner cannot review in Japanese.

### Japanese only

- Good, because the text that the owner reviews is the text that readers read.
- Bad, because most of the audience of a public template cannot read it.

### Japanese as the source, with an English translation next to each document

- Good, because the owner writes and reviews the text that is the authority.
- Bad, because the audience reads a translation, and wherever the translation is weak, the public text is weak.
- Bad, because code, comments, and commit messages are in English anyway, so the documents would be in a different language from the code they describe.

### A separate tree for the translations

- Good, because the English tree holds nothing but English.
- Bad, because a translation lives far from its source, so a change to one is easy to make without the other, and relative links differ between the two trees.
- Bad, because documents outside `docs/`, such as the README and `AGENTS.md`, would need a second scheme.

### Machine translation on demand

- Good, because nothing is written twice.
- Bad, because nobody reviews what the reader reads, and the terms change from one run to the next.
- Bad, because the owner's review of a translation leaves no trace in the repository.

## More Information

- [Translate a document](../../guides/translation.md) holds the rules and the table of terms.
- [Vale](https://vale.sh/), the [Google developer documentation style guide](https://developers.google.com/style), and [textlint](https://textlint.org/)
- The skill `sync-translation` and the subagent `translation-reviewer` are part of the harness in TPL-0020.
