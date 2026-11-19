# Terraform and Terragrunt (infra/): formatting, tflint with the AWS ruleset,
# the module READMEs (terraform-docs), and `terraform validate`. Trivy checks
# misconfiguration in make/security.mk. Settings: .tflint.hcl, .terraform-docs.yml.

SETUP_TARGETS += setup-tflint
FMT_TARGETS += fmt-terraform fmt-terragrunt
GEN_TARGETS += gen-terraform-docs
CHECK_TARGETS += fmt-check-terraform fmt-check-terragrunt lint-terraform lint-terraform-docs
CHECK_ALL_TARGETS += validate-terraform

# terraform fmt owns the files of the Terraform language; terragrunt hcl fmt
# owns every other HCL file (terragrunt.hcl, root.hcl, .tflint.hcl).
TF_GLOBS := "*.tf" "*.tfvars" "*.tftest.hcl"
HCL_GLOBS := "*.hcl" ":(exclude)*.tftest.hcl"
# Modules whose README.md terraform-docs maintains.
TF_DOCS_GLOBS := "infra/modules/*.tf" "infra/modules/*README.md"

TFLINT := tflint --config=$(CURDIR)/.tflint.hcl --format=json
TERRAFORM_DOCS := terraform-docs --config=$(CURDIR)/.terraform-docs.yml
TERRAGRUNT_FMT := terragrunt hcl fmt --no-tips --log-custom-format "%msg"

# The directories, among those of the files in scope, that hold a .tf file.
# A test file, tests/*.tftest.hcl, counts for the module that it tests.
tf-dirs = $(FILES) $(1) | xargs -r -n1 dirname | sed "s|/tests$$||" | sort -u | while read -r d; do \
	if [ -n "$$(find "$$d" -maxdepth 1 -name "*.tf" -print -quit)" ]; then echo "$$d"; fi; done

# Installs the tflint plugins that .tflint.hcl pins. Uses the GitHub API without
# a token; set GITHUB_TOKEN only if the rate limit is reached.
setup-tflint:
	tflint --init --config=$(CURDIR)/.tflint.hcl

fmt-terraform:
	@$(FILES) $(TF_GLOBS) | xargs -r terraform fmt -list=false

fmt-terragrunt:
	@$(FILES) $(HCL_GLOBS) | xargs -r -n1 $(TERRAGRUNT_FMT) --log-level error --file

gen-terraform-docs:
	@$(call tf-dirs,$(TF_DOCS_GLOBS)) | xargs -r -n1 $(TERRAFORM_DOCS) > /dev/null

fmt-check-terraform:
	@$(TASK) $@ '$(FILES) $(TF_GLOBS) | xargs -r terraform fmt -check -list=true -no-color \
		| sed "s|$$|:1: [terraform fmt] the file is not formatted. Fix: make fmt|"'

fmt-check-terragrunt:
	@$(TASK) $@ '$(FILES) $(HCL_GLOBS) | { s=0; while read -r f; do $(TERRAGRUNT_FMT) --check --file "$$f" 2>&1 \
		| sed "s|^File .* needs formatting$$|$$f:1: [terragrunt hcl fmt] the file is not formatted. Fix: make fmt|" || s=1; \
		done; exit $$s; }'

# Each module directory in scope, with the modules it calls from the repository.
lint-terraform:
	@$(TASK) $@ '$(call tf-dirs,$(TF_GLOBS)) | { s=0; while read -r d; do \
		$(TFLINT) --chdir="$$d" || s=1; done; exit $$s; } | jq -r -f tools/tflint/findings.jq'

lint-terraform-docs:
	@$(TASK) $@ '$(call tf-dirs,$(TF_DOCS_GLOBS)) | xargs -r -n1 $(TERRAFORM_DOCS) --output-check 2>&1 \
		| sed -E "s|^Error: (.*) is out of date$$|\1:1: [terraform-docs] the generated section is out of date. Fix: make gen|"'

# `terraform validate`, then the tests of the module (`terraform test` on
# tests/*.tftest.hcl), which plan against a mock of the AWS provider. Needs
# `terraform init`, which downloads the providers.
validate-terraform:
	@$(TASK) $@ '$(call tf-dirs,$(TF_GLOBS)) | xargs -r tools/scripts/terraform-validate.sh'
