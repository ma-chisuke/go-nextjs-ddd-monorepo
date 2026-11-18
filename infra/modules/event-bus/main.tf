# The event bus that carries the integration events of every bounded context
# (TPL-0013). A publishing context puts its events on the bus, and each
# consuming context adds a rule of its own (the module event-subscription), so
# the publisher does not know its consumers.

resource "aws_cloudwatch_event_bus" "this" {
  name = var.name
}
