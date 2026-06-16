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

data "foundrydb_organizations" "all" {}

locals {
  org_id = data.foundrydb_organizations.all.organizations[0].id
}

# PostgreSQL source cluster
resource "foundrydb_service" "pg_source" {
  name            = "prod-pg-source"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]
  organization_id = local.org_id
}

# Kafka sink cluster (must have the kafka-connect addon enabled)
resource "foundrydb_service" "kafka_sink" {
  name            = "prod-kafka-sink"
  database_type   = "kafka"
  version         = "3.9"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]
  organization_id = local.org_id
}

# CDC pipeline streaming selected tables from PostgreSQL into Kafka
resource "foundrydb_data_pipeline" "cdc" {
  organization_id  = local.org_id
  name             = "prod-pg-to-kafka"
  pipeline_type    = "cdc_pg_to_kafka"
  source_service_id = foundrydb_service.pg_source.id
  sink_service_id   = foundrydb_service.kafka_sink.id

  config {
    database_name = "app"
    tables        = ["public.orders", "public.users"]
    topic_prefix  = "cdc.prod"
    snapshot_mode = "initial"
  }
}

output "pipeline_id" {
  description = "UUID of the CDC pipeline"
  value       = foundrydb_data_pipeline.cdc.id
}

output "pipeline_status" {
  description = "Current status of the CDC pipeline"
  value       = foundrydb_data_pipeline.cdc.status
}

output "connector_name" {
  description = "Name of the Kafka Connect connector backing this pipeline"
  value       = foundrydb_data_pipeline.cdc.connector_name
}
