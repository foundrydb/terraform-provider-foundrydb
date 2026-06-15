terraform {
  required_providers {
    foundrydb = {
      source = "anorph/foundrydb"
    }
  }
}

provider "foundrydb" {
  api_url  = "https://api.foundrydb.com"
  username = "admin"
  password = "admin"
}

# A PostgreSQL service to host the queues
resource "foundrydb_service" "pg" {
  name            = "prod-pg"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]
}

# A task queue with default settings (30s visibility timeout, 5 attempts, DLQ enabled)
resource "foundrydb_queue" "tasks" {
  service_id = foundrydb_service.pg.id
  name       = "tasks"
}

# An events queue with custom settings and no dead-letter queue
resource "foundrydb_queue" "events" {
  service_id                 = foundrydb_service.pg.id
  name                       = "events"
  visibility_timeout_seconds = 60
  max_attempts               = 3
  dlq_enabled                = false
}

output "tasks_queue_id" {
  description = "UUID of the tasks queue"
  value       = foundrydb_queue.tasks.id
}

output "tasks_queue_status" {
  description = "Lifecycle status of the tasks queue"
  value       = foundrydb_queue.tasks.status
}

output "tasks_queue_database" {
  description = "Database where the queue schema objects live"
  value       = foundrydb_queue.tasks.database_name
}
