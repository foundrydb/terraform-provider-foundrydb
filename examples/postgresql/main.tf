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

# Provision a PostgreSQL 17 cluster on a tier-2 plan (2 CPU, 4 GB RAM)
resource "foundrydb_service" "postgres" {
  name            = "prod-pg"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]
}

# Fetch the admin user's credentials once the service is running
data "foundrydb_database_user" "app" {
  service_id = foundrydb_service.postgres.id
  username   = "admin"
}

output "service_id" {
  description = "UUID of the provisioned PostgreSQL service"
  value       = foundrydb_service.postgres.id
}

output "service_status" {
  description = "Current lifecycle status"
  value       = foundrydb_service.postgres.status
}

output "connection_string" {
  description = "Full connection string (sensitive)"
  value       = data.foundrydb_database_user.app.connection_string
  sensitive   = true
}
