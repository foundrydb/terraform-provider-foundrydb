# Terraform Provider for FoundryDB

Manage [FoundryDB](https://foundrydb.com) managed database clusters with Terraform. This provider supports PostgreSQL, MySQL, MongoDB, Valkey, and Kafka running on UpCloud infrastructure.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.0+
- [Go](https://golang.org/doc/install) 1.24+ (to build the provider from source)
- FoundryDB account with API access

## Installation

### From Terraform Registry

Add the provider to your Terraform configuration:

```hcl
terraform {
  required_providers {
    foundrydb = {
      source  = "anorph/foundrydb"
      version = "~> 1.0"
    }
  }
}
```

Then run `terraform init`.

### Build from Source

```bash
git clone https://github.com/anorph/terraform-provider-foundrydb.git
cd terraform-provider-foundrydb
CGO_ENABLED=0 go build -o terraform-provider-foundrydb .
```

Place the built binary in your [plugin directory](https://developer.hashicorp.com/terraform/cli/config/config-file#implied-local-mirror-directories).

## Provider Configuration

```hcl
provider "foundrydb" {
  api_url  = "https://api.foundrydb.com"   # optional, this is the default
  username = "admin"
  password = "admin"
}
```

### Provider Arguments

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `api_url` | string | No | Base URL of the FoundryDB API. Defaults to `https://api.foundrydb.com`. |
| `username` | string | Yes | Basic Auth username. |
| `password` | string | Yes | Basic Auth password. |

Use environment variables or a secrets manager to avoid hardcoding credentials.

## Resources

### `foundrydb_service`

Creates and manages a managed database service. After creation, the provider polls until the service reaches `running` status (up to 15 minutes).

#### Example

```hcl
resource "foundrydb_service" "postgres" {
  name            = "prod-pg"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]
}
```

#### Arguments

| Argument | Type | Required | Forces Replace | Description |
|----------|------|----------|----------------|-------------|
| `name` | string | Yes | No | Human-readable service name. |
| `database_type` | string | Yes | Yes | Engine: `postgresql`, `mysql`, `mongodb`, `valkey`, `kafka`. |
| `version` | string | No | Yes | Engine version, e.g. `17` for PostgreSQL 17. |
| `plan_name` | string | Yes | Yes | Compute tier: `tier-1` to `tier-15`. |
| `zone` | string | No | Yes | UpCloud zone. Default: `se-sto1` (Stockholm). |
| `storage_size_gb` | number | No | Yes | Data disk size in GB. |
| `storage_tier` | string | No | Yes | `maxiops` (NVMe SSD, production) or `standard` (HDD, development). |
| `allowed_cidrs` | list(string) | No | No | CIDR blocks allowed to connect to the service. |

#### Computed Attributes

| Attribute | Description |
|-----------|-------------|
| `id` | UUID of the service. |
| `status` | Current lifecycle status (e.g. `running`). |
| `created_at` | RFC3339 creation timestamp. |

#### Compute Tiers

| Tier | vCPUs | Memory |
|------|-------|--------|
| tier-1 | 1 | 4 GB |
| tier-2 | 2 | 4 GB |
| tier-3 | 2 | 8 GB |
| tier-4 | 4 | 8 GB |
| tier-5 | 4 | 16 GB |
| tier-6 | 8 | 32 GB |
| tier-7 | 8 | 64 GB |
| tier-8 | 16 | 64 GB |
| tier-9 | 20 | 96 GB |
| tier-10 | 24 | 128 GB |
| tier-11 | 32 | 128 GB |
| tier-12 | 40 | 160 GB |
| tier-13 | 48 | 224 GB |
| tier-14 | 64 | 256 GB |
| tier-15 | 80 | 512 GB |

#### In-place Updates

Only `name` and `allowed_cidrs` can be updated without recreating the service. All other arguments force replacement.

## Data Sources

### `foundrydb_database_user`

Retrieves the credentials for a database user. The `password` and `connection_string` attributes are marked sensitive.

#### Example

```hcl
data "foundrydb_database_user" "app" {
  service_id = foundrydb_service.postgres.id
  username   = "admin"
}

output "connection_string" {
  value     = data.foundrydb_database_user.app.connection_string
  sensitive = true
}
```

#### Arguments

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `service_id` | string | Yes | UUID of the managed service. |
| `username` | string | Yes | Database username to retrieve credentials for. |

#### Computed Attributes

| Attribute | Sensitive | Description |
|-----------|-----------|-------------|
| `password` | Yes | Database user password. |
| `host` | No | Hostname to connect to. |
| `port` | No | Port number. |
| `database` | No | Default database name. |
| `connection_string` | Yes | Full connection string for a database driver. |

## Complete Examples

### PostgreSQL

```hcl
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

resource "foundrydb_service" "postgres" {
  name            = "prod-pg"
  database_type   = "postgresql"
  version         = "17"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 50
  storage_tier    = "maxiops"
  allowed_cidrs   = ["0.0.0.0/0"]
}

data "foundrydb_database_user" "app" {
  service_id = foundrydb_service.postgres.id
  username   = "admin"
}

output "service_id" {
  value = foundrydb_service.postgres.id
}

output "connection_string" {
  value     = data.foundrydb_database_user.app.connection_string
  sensitive = true
}
```

### MySQL

```hcl
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
```

### MongoDB

```hcl
resource "foundrydb_service" "mongo" {
  name            = "prod-mongo"
  database_type   = "mongodb"
  version         = "7"
  plan_name       = "tier-4"
  zone            = "se-sto1"
  storage_size_gb = 200
  storage_tier    = "maxiops"
  allowed_cidrs   = ["10.0.0.0/8"]
}
```

### Valkey

```hcl
resource "foundrydb_service" "valkey" {
  name            = "prod-cache"
  database_type   = "valkey"
  version         = "8"
  plan_name       = "tier-2"
  zone            = "se-sto1"
  storage_size_gb = 20
  storage_tier    = "maxiops"
  allowed_cidrs   = ["10.0.0.0/8"]
}
```

### Kafka

```hcl
resource "foundrydb_service" "kafka" {
  name            = "prod-kafka"
  database_type   = "kafka"
  version         = "3.9"
  plan_name       = "tier-4"
  zone            = "se-sto1"
  storage_size_gb = 500
  storage_tier    = "maxiops"
  allowed_cidrs   = ["10.0.0.0/8"]
}
```

## Import

Existing services can be imported using their UUID:

```bash
terraform import foundrydb_service.postgres <service-uuid>
```

## License

MIT. See [LICENSE](LICENSE).
