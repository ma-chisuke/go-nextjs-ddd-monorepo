variable "name" {
  description = "The name of the consuming context, such as `notification`. The queue is named after it with the suffix `-events`, and its dead-letter queue with the suffix `-events-dlq`."
  type        = string
}

variable "event_bus_name" {
  description = "The name of the event bus that carries the events."
  type        = string
}

variable "sources" {
  description = "The sources of the events to receive: the CloudEvents `source` of each publishing context, such as `/contexts/reservation`."
  type        = list(string)
}

variable "event_types" {
  description = "The types of the events to receive: the CloudEvents `type` of each event, such as `reservation.ReservationConfirmed.v1`."
  type        = list(string)
}

variable "max_receive_count" {
  description = "How often the queue shows a message to the consumer before it moves the message to the dead-letter queue."
  type        = number
  default     = 5
}

variable "visibility_timeout_seconds" {
  description = "How long a received message stays hidden from other receives. It must exceed the time that the consumer gives the handling of one event (25 seconds by default, in `backend/internal/platform/awsevents`), or the event comes again while it is still being handled."
  type        = number
  default     = 60
}

variable "dead_letter_retention_seconds" {
  description = "How long the dead-letter queue keeps a message for someone to look at. The default is 14 days, the longest that SQS allows."
  type        = number
  default     = 1209600
}
