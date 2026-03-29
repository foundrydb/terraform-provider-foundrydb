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

# Provision a MySQL 8.4 cluster on a tier-3 plan (2 CPU, 8 GB RAM)
resource "foundrydb_service" "mysql" {
  name            = "prod-mysql"
  database_type   = "mysql"
  version         = "8.4"
  plan_name       = "tier-3"
  zone            = "se-sto1"
  storage_size_gb = 100
  storage_tier    = "maxiops"
  allowed_cidrs   = ["10.0.0.0/8"]
}

# Fetch the admin user's credentials
data "foundrydb_database_user" "app" {
  service_id = foundrydb_service.mysql.id
  username   = "admin"
}

output "service_id" {
  description = "UUID of the provisioned MySQL service"
  value       = foundrydb_service.mysql.id
}

output "service_status" {
  description = "Current lifecycle status"
  value       = foundrydb_service.mysql.status
}

output "db_host" {
  description = "Hostname to connect to"
  value       = data.foundrydb_database_user.app.host
}

output "db_port" {
  description = "Port to connect on"
  value       = data.foundrydb_database_user.app.port
}

output "connection_string" {
  description = "Full connection string (sensitive)"
  value       = data.foundrydb_database_user.app.connection_string
  sensitive   = true
}
