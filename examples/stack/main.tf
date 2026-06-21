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

# Read the catalog to discover available templates and their current prices.
data "foundrydb_stack_templates" "catalog" {}

output "available_templates" {
  description = "All available stack templates with estimated monthly costs"
  value = {
    for t in data.foundrydb_stack_templates.catalog.templates :
    t.name => {
      display_name  = t.display_name
      version       = t.version
      monthly_total = t.monthly_total
    }
  }
}

# Launch a RAG chatbot stack. The accepted_monthly_cost must match the
# monthly_total from the catalog within $0.01. Read it from the data source
# so the configuration stays in sync with platform pricing automatically.
locals {
  rag_template = one([
    for t in data.foundrydb_stack_templates.catalog.templates :
    t if t.name == "rag-chatbot"
  ])
}

resource "foundrydb_stack" "rag" {
  name                 = "prod-rag"
  template_name        = "rag-chatbot"
  accepted_monthly_cost = local.rag_template.monthly_total
}

output "stack_id" {
  description = "UUID of the provisioned stack"
  value       = foundrydb_stack.rag.id
}

output "stack_status" {
  description = "Lifecycle status of the stack"
  value       = foundrydb_stack.rag.status
}

output "stack_endpoint" {
  description = "Public URL of the stack's primary application (available once Running)"
  value       = foundrydb_stack.rag.endpoint_url
}

output "stack_monthly_cost" {
  description = "Estimated monthly cost as recorded at launch time"
  value       = foundrydb_stack.rag.estimated_monthly_cost
}

output "stack_resources" {
  description = "Child platform resources provisioned by the stack"
  value       = foundrydb_stack.rag.resources
}
