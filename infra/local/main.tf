# The AWS resources of the local environment (TPL-0017): the event bus, and
# the queue of each context that consumes events, in AWS Emulator. They come
# from the modules that the environments in AWS use, so the local queues and
# rules cannot drift from the real ones. `make up` applies this configuration
# every time, because the emulator keeps its state in memory
# (tools/scripts/aws.sh).

provider "aws" {
  region           = "us-east-1"
  custom_ca_bundle = var.ca_bundle

  # The emulator checks no signature and has no account to look up. The
  # credentials are placeholders from the environment (tools/scripts/aws.sh).
  skip_credentials_validation = true
  skip_metadata_api_check     = true

  endpoints {
    eventbridge = var.endpoint
    sqs         = var.endpoint
    sts         = var.endpoint
  }
}

module "event_bus" {
  source = "../modules/event-bus"

  name = "contexts"
}

# Notification sends mail to the organizer when a reservation is confirmed or
# cancelled (contracts/events/notification.asyncapi.yaml).
module "notification_events" {
  source = "../modules/event-subscription"

  name           = "notification"
  event_bus_name = module.event_bus.name
  sources        = ["/contexts/reservation"]
  event_types = [
    "reservation.ReservationConfirmed.v1",
    "reservation.ReservationCancelled.v1",
  ]
}
