package client

// ServiceCreateRequest is the request body for POST /managed-services/
type ServiceCreateRequest struct {
	Name          string   `json:"name"`
	DatabaseType  string   `json:"database_type"`
	PlanName      string   `json:"plan_name"`
	Version       string   `json:"version,omitempty"`
	Zone          string   `json:"zone,omitempty"`
	StorageSizeGB *int64   `json:"storage_size_gb,omitempty"`
	StorageTier   string   `json:"storage_tier,omitempty"`
	AllowedCIDRs  []string `json:"allowed_cidrs,omitempty"`
}

// ServiceUpdateRequest is the request body for PATCH /managed-services/{uuid}
type ServiceUpdateRequest struct {
	Name         *string  `json:"name,omitempty"`
	AllowedCIDRs []string `json:"allowed_cidrs,omitempty"`
}

// Service represents a managed database service from the API.
type Service struct {
	ID           string   `json:"uuid"`
	Name         string   `json:"name"`
	DatabaseType string   `json:"database_type"`
	PlanName     string   `json:"plan_name"`
	Version      string   `json:"version"`
	Zone         string   `json:"zone"`
	StorageSizeGB int64   `json:"storage_size_gb"`
	StorageTier  string   `json:"storage_tier"`
	AllowedCIDRs []string `json:"allowed_cidrs"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at"`
}

// ListServicesResponse is the response body for GET /managed-services/
type ListServicesResponse struct {
	Services []Service `json:"services"`
}

// DatabaseUser represents the revealed credentials for a database user.
type DatabaseUser struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	Host             string `json:"host"`
	Port             int64  `json:"port"`
	Database         string `json:"database"`
	ConnectionString string `json:"connection_string"`
}
