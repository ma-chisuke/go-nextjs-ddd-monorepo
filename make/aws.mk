# AWS Emulator, which stands in for AWS on this machine (TPL-0017). The image
# is built from infra/local/aws-emulator/, and compose.yaml runs it. The steps
# are in tools/scripts/aws.sh.

AWS := tools/scripts/aws.sh

# What the emulator is built from: a change to it starts the checks that run
# against the emulator.
AWS_SOURCES := "infra/local/aws-emulator/*" "tools/scripts/aws.sh"

.PHONY: aws-server

# Every target that needs the emulator waits for this one, so the emulator
# starts once, before them. It leaves a running emulator alone, as db-server
# does with PostgreSQL (make/db.mk).
aws-server:
	@$(TASK) $@ '$(AWS) server'

# The adapters for EventBridge and SQS run against the emulator in the tests
# behind the build tag `integration` (make/db.mk).
test-integration: | aws-server
