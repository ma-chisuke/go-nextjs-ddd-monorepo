# AWS Emulator, which stands in for AWS on this machine (TPL-0017). The image
# is built from infra/local/aws-emulator/, compose.yaml runs it, and Terraform
# creates the event bus and the queues in it from infra/local/. The steps are
# in tools/scripts/aws.sh.

AWS := tools/scripts/aws.sh

# What the emulator is built from and filled with: a change to it starts the
# checks that run against the emulator.
AWS_SOURCES := "infra/local/*" "infra/modules/*" "tools/scripts/aws.sh"

.PHONY: aws-server aws-apply

# Every target that needs the emulator waits for this one, so the emulator
# starts once, before them. It leaves a running emulator alone, as db-server
# does with PostgreSQL (make/db.mk). It starts the emulator only when a file
# in scope concerns a check that uses it, so a change to the documents alone
# builds no image.
aws-server:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(GO_GLOBS) "*.sql" $(STACK_SOURCES) $(AWS_SOURCES))" ]; then $(AWS) server; fi'

aws-apply: ## Create the event bus and the queues in the running AWS Emulator
	@$(AWS) apply

# The adapters for EventBridge and SQS run against the emulator in the tests
# behind the build tag `integration` (make/db.mk), and the smoke test and the
# end-to-end test send their events through it (make/stack.mk).
test-integration smoke e2e: | aws-server
