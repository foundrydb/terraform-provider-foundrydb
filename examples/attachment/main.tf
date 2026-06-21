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

# A PostgreSQL database that the companion app will connect to
resource "foundrydb_service" "db" {
  name            = "analytics-pg"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
}

# Attach Metabase to the PostgreSQL service as a companion analytics app.
# The platform provisions an app service, peers it to the database over a
# private SDN, and serves it at the subdomain over auto-TLS HTTPS.
resource "foundrydb_attachment" "metabase" {
  parent_service_id = foundrydb_service.db.id
  kind              = "metabase"
  plan_name         = "tier-2"
  subdomain         = "analytics"
}

output "attachment_id" {
  description = "Attachment identifier"
  value       = foundrydb_attachment.metabase.id
}

output "companion_app_service_id" {
  description = "UUID of the underlying app service for the companion app"
  value       = foundrydb_attachment.metabase.app_service_id
}

output "companion_app_url" {
  description = "Public HTTPS URL of the Metabase companion app"
  value       = foundrydb_attachment.metabase.url
}

output "companion_app_status" {
  description = "Lifecycle status of the companion app"
  value       = foundrydb_attachment.metabase.status
}
