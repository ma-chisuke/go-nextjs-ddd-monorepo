---
status: accepted
date: 2026-11-17
---

# TPL-0019: License the template under MIT-0

## Context and Problem Statement

People copy a template and make it their own product: they rename it, rewrite most of it, and ship it.
The license of a template therefore matters less for the template itself than for every product that starts from it.

The MIT License, the usual choice for a permissive license, allows all of this on one condition: the copyright notice and the permission notice must be included in all copies or substantial portions of the software.
For a template, that condition follows the code into every product made from it, long after the product has stopped resembling the template.
In practice, people delete the notice along with the rest of the template's identity, and their product is then out of compliance with the license without anyone noticing.

Which license lets a team take the template as its own, without a condition that most teams would break?

## Decision Drivers

- A team must be able to use, change, and ship the template without keeping a notice for the author of the template.
- Legal reviewers and license scanners must recognize the license: it must be approved by the OSI and have an SPDX identifier.
- A reader should see at once what the license allows, so its wording should be familiar.
- The license must work the same way in every country.

## Considered Options

- MIT-0 (MIT No Attribution)
- MIT
- Apache-2.0
- A dedication to the public domain (CC0-1.0 or the Unlicense)
- 0BSD

## Decision Outcome

Chosen option: "MIT-0", because it is the MIT License without the condition, so it gives teams the freedom they need in wording that readers already know.
AWS publishes its sample code under MIT-0 for the same reason: people copy samples into their own code.

The rules are as follows.

- `LICENSE` holds the text of MIT-0, and `package.json` declares `"license": "MIT-0"`.
- A dependency under a strong copyleft license (GPL, AGPL) or under a source-available license (SSPL, BUSL, the Commons Clause, the Creative Commons licenses with NonCommercial) must not become part of a product built from the template. `trivy.yaml` lists these licenses as forbidden, and the license scan fails on them only. Weak copyleft, such as LGPL and MPL, is allowed, because it covers the library itself and not the code that uses it; for example, `next` brings libvips, under LGPL-3.0, through sharp.
- Third-party files kept in the repository keep their own licenses, stated in or next to the files. The Google style for Vale in `.vale/styles/Google/` is under the MIT License, and the Japanese writing norm in `.claude/skills/japanese-tech-writing/` is under the Unlicense.

### Consequences

- Good, because a team can remove every trace of the template from its product and still comply with the license.
- Good, because MIT-0 is approved by the OSI and has the SPDX identifier `MIT-0`, so scanners and legal reviewers recognize it.
- Bad, because MIT-0, like MIT, has no patent grant. Apache-2.0 has one.
- Bad, because a team that needs a dependency under a forbidden license has to replace this decision with a record of its own, and change `trivy.yaml`.
- Bad, because the files under other licenses are exceptions that a team must notice: a copy that keeps them keeps their notices too.
- Bad, because a product that ships a library under a weak copyleft license, such as libvips under LGPL-3.0, must still meet the terms of that license for the library itself.

### Confirmation

- Review confirms that `LICENSE` and `package.json` name MIT-0.
- `make check-all` runs the target `scan-deps`, which fails when Trivy finds a dependency under a license that `trivy.yaml` forbids. The finding names this record (`tools/trivy/findings.tpl`). The scan covers what ships with the product: development dependencies and the `tools/` module are left out.
- `trivy.yaml` sets `license.full: false`, so Trivy reads the license that each package declares, not the license text of each file. A file copied into the repository by hand is out of its reach, and review confirms that the license of such a file is stated in or next to it.

## Pros and Cons of the Options

### MIT-0

- Good, because it removes the one condition of MIT that the users of a template would break.
- Good, because its text is the text of MIT without the condition, so a reader who knows MIT reads it at once.
- Bad, because it is less known than MIT, and some readers will look it up.
- Bad, because it has no patent grant.

### MIT

- Good, because it is the most widely known permissive license.
- Bad, because every product made from the template must keep the copyright notice of the template, and most teams that remove the template's identity remove the notice too.

### Apache-2.0

- Good, because it grants a license to the patents of contributors, and ends that license for anyone who sues over those patents.
- Bad, because it has more conditions than MIT, not fewer: a copy must include the license, keep the notices, mark the files it changed, and carry the `NOTICE` file if there is one.

### A dedication to the public domain

- Good, because it asks for nothing at all.
- Bad, because the public domain does not work the same way in every country, and each text deals with that in its own words (CC0-1.0 with a fallback license). Legal reviewers know these texts less well than MIT.
- Bad, because CC0-1.0 states that it grants no patent rights, and the OSI has not approved it.

### 0BSD

- Good, because it also has no condition of attribution, and the OSI has approved it.
- Bad, because its name suggests a BSD license, while its text comes from the ISC license. MIT-0 keeps the wording that most readers know.

## More Information

- [MIT-0 on the SPDX License List](https://spdx.org/licenses/MIT-0.html)
- [MIT-0 as published by AWS](https://github.com/aws/mit-0)
- The license scan and the other defenses of the supply chain are in TPL-0024.
