# github-oidc

The roles that let GitHub Actions reach one AWS account without a stored key (TPL-0018).
A job asks GitHub for a short-lived token, and AWS exchanges it for the credentials of a role when the trust policy of the role names the subject of the token.

| Role | Who may assume it | What it may do |
|---|---|---|
| `github-plan` | A job of a pull request of the repository | Read the infrastructure (`ReadOnlyAccess`), and lock the state |
| `github-deploy` | A job that names the GitHub Environment of this environment | Apply changes (`deploy_policy_arns`) |

Each account, stg and prd, gets its own two roles, so the credentials of one environment are worth nothing in the other (TPL-0015).

- Both trust policies compare the whole subject with `StringEquals`, and `repository` must name one repository without a wildcard. A condition that matches too much, such as every repository of an owner, would give the role to code that nobody reviewed.
- The settings of the GitHub Environment decide which branch may deploy and whether a person approves the job first. For prd, require a reviewer ([GitHub settings](../../../docs/guides/github-settings.md)).
- GitHub gives no token to a pull request from a fork, so such a pull request gets no plan.
- The role that deploys has `AdministratorAccess` by default. It creates IAM roles, so a narrower policy would not limit what it can reach; what limits it is who may assume it.

Someone with administrator access applies this module once for each account, because the workflows cannot assume a role that does not exist yet. After that, a change to it goes through a pull request like any other.

Not yet applied to AWS: the subjects follow the documentation of GitHub. For the immutable form of a repository, the documentation shows the subject of a branch only; the subjects of a pull request and of an Environment are built the same way here. If AWS refuses the token, compare the outputs `plan_subject` and `deploy_subject` with the `sub` claim of the token.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.17 |
| aws | >= 6.0 |

## Providers

| Name | Version |
| ---- | ------- |
| aws | >= 6.0 |

## Modules

No modules.

## Resources

| Name | Type |
| ---- | ---- |
| [aws_iam_openid_connect_provider.github](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_openid_connect_provider) | resource |
| [aws_iam_role.deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.plan](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.plan_lock](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_iam_role_policy_attachment.deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy_attachment) | resource |
| [aws_iam_role_policy_attachment.plan_read_only](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy_attachment) | resource |
| [aws_iam_openid_connect_provider.github](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_openid_connect_provider) | data source |
| [aws_iam_policy_document.deploy_assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.plan_assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.plan_lock](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_partition.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/partition) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| create\_provider | Whether to create the OIDC identity provider for GitHub in this account. An account holds one provider for each URL: set this to `false` when the account already has it. | `bool` | `true` | no |
| deploy\_policy\_arns | The policies of the role that deploys. The default is `AdministratorAccess`: a role that applies this infrastructure creates IAM roles, so a narrower policy would not limit what it can reach. What limits the role is who may assume it: the jobs of one GitHub Environment of one repository. | `list(string)` | ```[ "arn:aws:iam::aws:policy/AdministratorAccess" ]``` | no |
| environment | The name of the environment, such as `stg`. It is also the name of the GitHub Environment whose jobs may assume the role that deploys. | `string` | n/a | yes |
| repository | The repository whose workflows may assume the roles, written as GitHub writes it in the subject of its tokens. For a repository created before July 15, 2026 that is `owner/name`. A repository created later, renamed, or transferred uses the immutable form with both IDs, `owner@owner-id/name@repository-id`; `gh api repos/OWNER/NAME --jq '"\(.owner.login)@\(.owner.id)/\(.name)@\(.id)"'` prints it. | `string` | n/a | yes |
| state\_bucket | The name of the S3 bucket that holds the Terraform state of this environment. The role that plans may write and delete the lock files in it, and nothing else. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| deploy\_role\_arn | The ARN of the role that applies changes, for the workflow that deploys. |
| deploy\_subject | The subject that a token must have to assume the role that applies changes. |
| plan\_role\_arn | The ARN of the read-only role, for the workflow that plans on pull requests. |
| plan\_subject | The subject that a token must have to assume the read-only role. |
<!-- END_TF_DOCS -->
