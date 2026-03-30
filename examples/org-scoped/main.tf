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

# List all organizations the authenticated user belongs to
data "foundrydb_organizations" "all" {}

output "organizations" {
  description = "All organizations for the authenticated user"
  value       = data.foundrydb_organizations.all.organizations
}

# Provision a PostgreSQL 17 service scoped to a specific organization.
# All API calls for this resource will include X-Active-Org-ID header.
resource "foundrydb_service" "org_postgres" {
  name            = "org-prod-pg"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]

  # Set this to the ID of the organization from foundrydb_organizations data source
  # or provide the org ID directly as a string.
  organization_id = data.foundrydb_organizations.all.organizations[0].id
}

# Fetch credentials for the org-scoped service
data "foundrydb_database_user" "app" {
  service_id = foundrydb_service.org_postgres.id
  username   = "admin"
}

output "service_id" {
  description = "UUID of the org-scoped PostgreSQL service"
  value       = foundrydb_service.org_postgres.id
}

output "connection_string" {
  description = "Full connection string (sensitive)"
  value       = data.foundrydb_database_user.app.connection_string
  sensitive   = true
}
