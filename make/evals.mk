# The evaluation of the agent harness (evals/, TPL-0020): a coding agent does
# a task of this repository with the harness and without it, and a grader
# compares the results. The steps are in evals/run.sh.

CHECK_ALL_TARGETS += test-evals

EVALS := evals/run.sh

.PHONY: eval eval-report

# The graders, without an agent: for each task, the repository as it is
# fails, and with the solution of the task it passes, `make check` included.
# A solution names the files that it changes, so the target runs when one of
# those is in scope, or a file of evals/: a change to the code under a
# solution must not leave the solution behind.
test-evals:
	@$(TASK) $@ 'if [ -n "$$($(FILES) "evals/*" $$(sed -n "s|^+++ b/||p" evals/tasks/*/solution.patch 2> /dev/null | sort -u))" ]; then $(EVALS) verify; fi'

# The targets below start a coding agent, which costs time and money, so no
# check runs them.

eval: ## Let the agent do a task and grade the result (EVAL_TASK=read-the-logs EVAL_ARM=harness, or EVAL_ARM=bare)
	@$(EVALS) run "$(EVAL_TASK)" "$(EVAL_ARM)"

eval-report: ## A table of the evaluation runs recorded in evals/results/
	@$(EVALS) report
