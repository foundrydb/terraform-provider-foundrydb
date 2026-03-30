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

# Provision an OpenSearch 2.19 cluster on a tier-4 plan (4 CPU, 8 GB RAM)
resource "foundrydb_service" "opensearch" {
  name            = "prod-opensearch"
  database_type   = "opensearch"
  version         = "2.19"
  plan_name       = "tier-4"
  zone            = "se-sto1"
  storage_size_gb = 200
  storage_tier    = "maxiops"
  allowed_cidrs   = ["10.0.0.0/8"]
}

output "service_id" {
  description = "UUID of the provisioned OpenSearch service"
  value       = foundrydb_service.opensearch.id
}

output "service_status" {
  description = "Current lifecycle status"
  value       = foundrydb_service.opensearch.status
}
