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

# Provision an app service to attach jobs to
resource "foundrydb_service" "app" {
  name            = "prod-app"
  database_type   = "app"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 20
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]
}

# A scheduled job that runs every night at 02:00 Stockholm time
resource "foundrydb_app_job" "nightly_cleanup" {
  app_service_id = foundrydb_service.app.id
  name           = "nightly-cleanup"
  schedule_cron  = "0 2 * * *"
  timezone       = "Europe/Stockholm"
  enabled        = true

  # Override the app image for this specific job
  image_ref = "registry.example.com/tools:cleanup-v2"

  # Override the container command
  command = ["/app/cleanup", "--older-than=30d"]

  # Extra environment variables layered over the app's environment
  env = {
    CLEANUP_DRY_RUN = "false"
    LOG_LEVEL       = "info"
  }

  max_retries          = 2
  retry_backoff_seconds = 60
  max_runtime_seconds  = 1800
  concurrency_cap      = 1
}

# An unscheduled job triggered manually via RunAppJob
resource "foundrydb_app_job" "data_export" {
  app_service_id = foundrydb_service.app.id
  name           = "data-export"

  command = ["/app/export", "--format=csv"]

  env = {
    EXPORT_BUCKET = "s3://my-exports"
  }

  max_runtime_seconds = 3600
  concurrency_cap     = 1
}

output "nightly_cleanup_job_id" {
  description = "UUID of the nightly cleanup job"
  value       = foundrydb_app_job.nightly_cleanup.id
}
