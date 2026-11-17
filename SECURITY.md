# Security policy

## Supported versions

Fixes go into the latest release of the template and into `main`.
Older releases get no fixes.

| Version | Supported |
|---|---|
| Latest release | Yes |
| Older releases | No |

A repository created from the template is a project of its own and does not receive fixes by itself.
Its team reads the release notes and takes the fix into its copy.

## Report a vulnerability

Report a vulnerability privately through GitHub, never in a public issue, pull request, or discussion:

1. Open the **Security** tab of this repository.
2. Select **Report a vulnerability**.
3. Describe the problem: what an attacker can do, the files or workflows involved, and the steps to reproduce it.

GitHub keeps the report visible only to you and to the maintainers.
One person maintains this template in their spare time. The maintainer aims to answer in the report within two weeks, agrees on a fix and a date for disclosure with you, and credits you in the published advisory unless you ask otherwise.
GitHub explains the steps in [Privately reporting a security vulnerability](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability).

## Scope

This policy covers what the template itself contains: its workflows, scripts, configuration, and code.

- A vulnerability in a project built from the template belongs to the team of that project.
- A vulnerability in a tool or a dependency belongs to its maintainers. If the template uses the affected version, report it here as well, so that the template can move to a fixed one.
