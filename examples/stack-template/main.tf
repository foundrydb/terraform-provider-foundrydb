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

# Define a customer-authored stack template. The descriptor declares a
# PostgreSQL database and a companion app service that together provide a
# lightweight analytics environment.
resource "foundrydb_stack_template" "analytics" {
  name         = "custom-analytics-stack"
  display_name = "Custom Analytics Stack"
  description  = "PostgreSQL + analytics app service for internal dashboards."
  version      = "1.0.0"
  visibility   = "org_shared"

  # descriptor is a JSON-encoded StackDescriptor. The platform validates it on
  # create and update. At minimum, each resource entry needs a name, kind, and
  # spec block appropriate for that kind.
  descriptor = jsonencode({
    apiVersion  = "stacks/v1"
    name        = "custom-analytics-stack"
    displayName = "Custom Analytics Stack"
    description = "PostgreSQL + analytics app for internal dashboards"
    version     = "1.0.0"
    resources = [
      {
        name = "db"
        kind = "database"
        spec = {
          database_type   = "postgresql"
          version         = "17"
          plan_name       = "tier-2"
          zone            = "se-sto1"
          storage_size_gb = 50
          storage_tier    = "maxiops"
        }
      },
      {
        name = "app"
        kind = "app"
        spec = {
          plan_name = "tier-2"
          image_ref = "registry.example.com/analytics:latest"
          zone      = "se-sto1"
          env = {
            LOG_LEVEL = "info"
          }
        }
      }
    ]
    dependencies = {
      app = ["db"]
    }
  })

  # Set to true to share the template within the owning organization immediately
  # after creation. For visibility = "public", this submits to the moderation queue.
  publish = true
}

output "template_id" {
  description = "UUID of the published custom template"
  value       = foundrydb_stack_template.analytics.id
}

output "template_publication_status" {
  description = "Moderation lifecycle of the template (draft, submitted, published, etc.)"
  value       = foundrydb_stack_template.analytics.publication_status
}

output "template_organization_id" {
  description = "Organization that owns this template"
  value       = foundrydb_stack_template.analytics.organization_id
}

# Launch a stack from the custom template. First preview the cost, then pass
# the accepted amount to satisfy the platform cost gate.
resource "foundrydb_stack" "analytics_instance" {
  name        = "team-analytics"
  template_id = foundrydb_stack_template.analytics.id

  # The platform computes the cost from the descriptor at launch time.
  # For custom templates, use the PreviewStackCost API call or the
  # foundrydb CLI (fdb stack preview --template-id <id>) to get the amount
  # to pass here.
  accepted_monthly_cost = 50.00

  depends_on = [foundrydb_stack_template.analytics]
}

output "stack_id" {
  description = "UUID of the launched stack instance"
  value       = foundrydb_stack.analytics_instance.id
}

output "stack_status" {
  description = "Lifecycle status of the stack"
  value       = foundrydb_stack.analytics_instance.status
}

output "stack_endpoint" {
  description = "Public URL of the stack's primary app endpoint (available once Running)"
  value       = foundrydb_stack.analytics_instance.endpoint_url
}
