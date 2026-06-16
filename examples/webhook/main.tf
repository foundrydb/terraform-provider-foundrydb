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

# Fetch the organization to get its ID
data "foundrydb_organizations" "all" {}

locals {
  org_id = data.foundrydb_organizations.all.organizations[0].id
}

# A webhook endpoint subscribed to service lifecycle events
resource "foundrydb_webhook" "service_events" {
  organization_id = local.org_id
  url             = "https://hooks.example.com/foundrydb"
  events          = ["service.created", "service.deleted", "service.status_changed"]
}

# A webhook endpoint subscribed to all events
resource "foundrydb_webhook" "all_events" {
  organization_id = local.org_id
  url             = "https://hooks.example.com/foundrydb-all"
  # omitting events subscribes to every event type
}

output "service_webhook_id" {
  description = "UUID of the service-events webhook endpoint"
  value       = foundrydb_webhook.service_events.id
}

output "service_webhook_secret" {
  description = "HMAC signing secret for the service-events webhook (store securely)"
  value       = foundrydb_webhook.service_events.secret
  sensitive   = true
}
