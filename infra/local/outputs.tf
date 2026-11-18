output "event_bus_name" {
  description = "The name of the event bus: EVENT_BUS_NAME of the relays in compose.yaml."
  value       = module.event_bus.name
}

output "notification_events_queue_url" {
  description = "The URL of the queue of Notification: EVENTS_QUEUE_URL of its worker in compose.yaml."
  value       = module.notification_events.queue_url
}
