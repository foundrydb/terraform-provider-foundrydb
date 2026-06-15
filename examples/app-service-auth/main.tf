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
  password = var.foundrydb_password
}

variable "foundrydb_password" {
  description = "FoundryDB admin password."
  sensitive   = true
}

variable "smtp_password" {
  description = "SMTP relay password used to send magic-link emails."
  sensitive   = true
}

variable "google_client_secret" {
  description = "Google OAuth client secret for Sign in with Google."
  sensitive   = true
}

# PostgreSQL backing database for the identity store.
resource "foundrydb_service" "identity_db" {
  name            = "myapp-identity"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 25
  storage_tier    = "maxiops"
}

# App service that serves the customer-facing application.
# (foundrydb_app_service is managed separately; this example imports its ID.)
locals {
  app_service_id = "YOUR_APP_SERVICE_UUID"
  attachment_id  = "YOUR_ATTACHMENT_UUID"
}

# Enable end-user authentication backed by the PostgreSQL identity database.
# The SMTP credentials and the Google client secret are stored in the platform
# secret store and never returned by subsequent API reads.
resource "foundrydb_app_service_auth" "myapp" {
  app_service_id      = local.app_service_id
  attachment_id       = local.attachment_id
  issuer_domain_choice = "fallback"

  smtp = {
    host         = "smtp.mailgun.org"
    port         = 587
    username     = "postmaster@mg.example.com"
    password     = var.smtp_password
    from_address = "noreply@example.com"
    from_name    = "My App"
  }

  theme = {
    display_name = "My App"
    brand_color  = "#4F46E5"
    logo_url     = "https://example.com/logo.png"
    support_url  = "https://example.com/support"
  }

  idp_providers = [
    {
      provider      = "google"
      client_id     = "123456789-abc.apps.googleusercontent.com"
      client_secret = var.google_client_secret
      display_name  = "Google"
    }
  ]
}

output "issuer_url" {
  description = "OIDC issuer URL to configure in your application."
  value       = foundrydb_app_service_auth.myapp.issuer_url
}

output "auth_status" {
  description = "Current provisioning status of the auth configuration."
  value       = foundrydb_app_service_auth.myapp.status
}
