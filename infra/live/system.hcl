# What every environment has in common. The file of each environment,
# <environment>/env.hcl, holds what differs.

locals {
  # A short name of the system, which starts every name that must be unique
  # in the world, such as the bucket of the Terraform state.
  system = "meeting-rooms"

  # The repository whose workflows may reach the AWS accounts, as GitHub
  # writes it in the subject of its tokens (infra/modules/github-oidc).
  repository = "ma-chisuke@25313731/go-nextjs-ddd-monorepo@1461588963"

  # The bounded contexts: each has a directory in backend/cmd/, and a unit
  # in <environment>/contexts/. `make lint-infra-live` compares the three.
  contexts = ["facility", "notification", "reservation"]

  # The name of the event bus, as in the local system (infra/local).
  event_bus_name = "contexts"
}
