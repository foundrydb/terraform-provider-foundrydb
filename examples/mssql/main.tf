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

# Provision an MSSQL 4.8 cluster on a tier-4 plan (4 CPU, 8 GB RAM)
resource "foundrydb_service" "mssql" {
  name            = "prod-mssql"
  database_type   = "mssql"
  version         = "4.8"
  plan_name       = "tier-4"
  zone            = "se-sto1"
  storage_size_gb = 100
  storage_tier    = "maxiops"
  allowed_cidrs   = ["10.0.0.0/8"]
}

# Fetch the admin user's credentials
data "foundrydb_database_user" "app" {
  service_id = foundrydb_service.mssql.id
  username   = "admin"
}

output "service_id" {
  description = "UUID of the provisioned MSSQL service"
  value       = foundrydb_service.mssql.id
}

output "service_status" {
  description = "Current lifecycle status"
  value       = foundrydb_service.mssql.status
}

output "connection_string" {
  description = "Full connection string (sensitive)"
  value       = data.foundrydb_database_user.app.connection_string
  sensitive   = true
}
