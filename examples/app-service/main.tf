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

# A PostgreSQL database the app will use
resource "foundrydb_service" "db" {
  name            = "prod-pg"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]
}

# An app service running a container with the database attached
resource "foundrydb_app_service" "api" {
  name      = "prod-api"
  plan_name = "tier-2"
  zone      = "se-sto1"

  app_config {
    image_ref      = "registry.example.com/myapp:latest"
    container_port = 8080

    env = {
      LOG_LEVEL = "info"
      APP_ENV   = "production"
    }

    health_check_path             = "/healthz"
    health_check_interval_seconds = 30
    health_check_timeout_seconds  = 5
    health_check_healthy_threshold = 2
  }

  attached_service_ids = [foundrydb_service.db.id]
}

output "app_service_id" {
  description = "UUID of the app service"
  value       = foundrydb_app_service.api.id
}

output "app_service_status" {
  description = "Lifecycle status of the app service"
  value       = foundrydb_app_service.api.status
}
