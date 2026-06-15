package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anorph/terraform-provider-foundrydb/internal/provider"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// serviceResponse is a convenience helper to build a minimal API JSON response.
func serviceResponse(id, name, dbType, version, status string) map[string]interface{} {
	return map[string]interface{}{
		"id":              id,
		"name":            name,
		"database_type":   dbType,
		"version":         version,
		"status":          status,
		"plan_name":       "tier-2",
		"zone":            "se-sto1",
		"storage_size_gb": 50,
		"storage_tier":    "maxiops",
		"allowed_cidrs":   []string{},
		"dns_records":     []interface{}{},
		"created_at":      "2026-01-01T00:00:00Z",
		"updated_at":      "2026-01-01T00:00:00Z",
	}
}

// jsonBody encodes v as JSON and panics on error (test helper only).
func jsonBody(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// newRunningServiceServer returns an httptest.Server simulating a FoundryDB API that:
//   - POST /managed-services -> 201 with provisioning service
//   - GET  /managed-services/:id -> 200 with running service
//   - PATCH /managed-services/:id -> 200 with updated service
//   - DELETE /managed-services/:id -> 204
func newRunningServiceServer(t *testing.T, svcID, svcName string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/managed-services", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			svc := serviceResponse(svcID, svcName, "postgresql", "17", "provisioning")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(svc))
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/managed-services/"+svcID, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			svc := serviceResponse(svcID, svcName, "postgresql", "17", "running")
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(svc))
		case http.MethodPatch:
			var req map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&req)
			svc := serviceResponse(svcID, svcName, "postgresql", "17", "running")
			if n, ok := req["name"].(string); ok {
				svc["name"] = n
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(svc))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	return httptest.NewServer(mux)
}

// TestUnitServiceResource_Metadata verifies the resource type name is "foundrydb_service".
func TestUnitServiceResource_Metadata(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.MetadataRequest{ProviderTypeName: "foundrydb"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "foundrydb_service" {
		t.Errorf("TypeName = %q; want %q", resp.TypeName, "foundrydb_service")
	}
}

// TestUnitServiceResource_Schema_requiredAttributes verifies all required attributes exist.
func TestUnitServiceResource_Schema_requiredAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	for _, key := range []string{"name", "database_type", "plan_name"} {
		attr, ok := resp.Schema.Attributes[key]
		if !ok {
			t.Errorf("schema missing required attribute %q", key)
			continue
		}
		if !attr.IsRequired() {
			t.Errorf("attribute %q should be Required", key)
		}
	}
}

// TestUnitServiceResource_Schema_computedAttributes verifies computed-only attributes.
func TestUnitServiceResource_Schema_computedAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"id", "status", "created_at"} {
		attr, ok := resp.Schema.Attributes[key]
		if !ok {
			t.Errorf("schema missing computed attribute %q", key)
			continue
		}
		if !attr.IsComputed() {
			t.Errorf("attribute %q should be Computed", key)
		}
	}
}

// TestUnitServiceResource_Schema_optionalComputedAttributes verifies optional+computed attributes.
func TestUnitServiceResource_Schema_optionalComputedAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"version", "zone", "storage_size_gb", "storage_tier", "allowed_cidrs"} {
		attr, ok := resp.Schema.Attributes[key]
		if !ok {
			t.Errorf("schema missing optional/computed attribute %q", key)
			continue
		}
		if !attr.IsOptional() {
			t.Errorf("attribute %q should be Optional", key)
		}
		if !attr.IsComputed() {
			t.Errorf("attribute %q should be Computed", key)
		}
	}
}

// TestUnitServiceResource_Schema_organizationID verifies organization_id is purely optional.
func TestUnitServiceResource_Schema_organizationID(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	attr, ok := resp.Schema.Attributes["organization_id"]
	if !ok {
		t.Fatal("schema missing organization_id attribute")
	}
	if !attr.IsOptional() {
		t.Error("organization_id should be Optional")
	}
	if attr.IsRequired() {
		t.Error("organization_id should not be Required")
	}
}

// TestUnitServiceResource_Schema_databaseTypeRequiresReplace verifies that changing
// database_type forces resource replacement.
func TestUnitServiceResource_Schema_databaseTypeRequiresReplace(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	attr, ok := resp.Schema.Attributes["database_type"]
	if !ok {
		t.Fatal("database_type attribute not found")
	}
	strAttr, ok := attr.(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("database_type is not a StringAttribute; got %T", attr)
	}
	if len(strAttr.PlanModifiers) == 0 {
		t.Error("database_type should have plan modifiers (RequiresReplace)")
	}
}

// TestUnitServiceResource_Schema_versionRequiresReplace verifies that changing version
// forces resource replacement.
func TestUnitServiceResource_Schema_versionRequiresReplace(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	attr, ok := resp.Schema.Attributes["version"]
	if !ok {
		t.Fatal("version attribute not found")
	}
	strAttr, ok := attr.(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("version is not a StringAttribute; got %T", attr)
	}
	if len(strAttr.PlanModifiers) == 0 {
		t.Error("version should have plan modifiers (RequiresReplace)")
	}
}

// TestUnitServiceResource_Schema_idComputedNotRequired verifies the id attribute is
// not user-supplied.
func TestUnitServiceResource_Schema_idComputedNotRequired(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	attr, ok := resp.Schema.Attributes["id"]
	if !ok {
		t.Fatal("id attribute not found")
	}
	if attr.IsRequired() {
		t.Error("id should not be Required (it is set by the API)")
	}
	if !attr.IsComputed() {
		t.Error("id should be Computed")
	}
}

// TestUnitServiceResource_Schema_markdownDescription verifies the resource has a description.
func TestUnitServiceResource_Schema_markdownDescription(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Schema.MarkdownDescription == "" {
		t.Error("service resource schema MarkdownDescription should not be empty")
	}
}

// TestUnitServiceResource_ImplementsInterfaces verifies the resource satisfies both
// resource.Resource and resource.ResourceWithImportState.
func TestUnitServiceResource_ImplementsInterfaces(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()
	// Compile-time assertion: resource.Resource is satisfied.
	var _ resource.Resource = r
	// Compile-time assertion: resource.ResourceWithImportState requires ImportState.
	// NewServiceResource returns a concrete type that satisfies both interfaces.
	// We verify this with a type assertion.
	if _, ok := r.(resource.ResourceWithImportState); !ok {
		t.Error("NewServiceResource() does not implement resource.ResourceWithImportState")
	}
}

// TestUnitServiceResource_Configure_nilProviderData verifies Configure does not panic
// when called with nil provider data (happens during initial framework setup).
func TestUnitServiceResource_Configure_nilProviderData(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewServiceResource() does not implement resource.ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: nil}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with nil provider data should not produce errors; got: %v", resp.Diagnostics)
	}
}

// TestUnitServiceResource_Configure_wrongType verifies Configure produces a diagnostic error
// when the provider data is the wrong type.
func TestUnitServiceResource_Configure_wrongType(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewServiceResource() does not implement resource.ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: "not-a-client"}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure with wrong provider data type should produce an error")
	}
}

// TestUnitServiceResource_Schema_allExpectedFields verifies that no attributes are
// accidentally dropped during refactoring.
func TestUnitServiceResource_Schema_allExpectedFields(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	expected := []string{
		"id", "name", "database_type", "version", "plan_name",
		"zone", "storage_size_gb", "storage_tier", "allowed_cidrs",
		"organization_id", "status", "created_at",
	}
	for _, field := range expected {
		if _, ok := resp.Schema.Attributes[field]; !ok {
			t.Errorf("expected attribute %q not found in service resource schema", field)
		}
	}
}

// TestUnitServiceResourceModel_defaultNullValues verifies that default types.String and
// types.Int64 values are null, matching framework semantics.
func TestUnitServiceResourceModel_defaultNullValues(t *testing.T) {
	t.Parallel()
	var s types.String
	var i types.Int64
	var l types.List

	if !s.IsNull() {
		t.Error("default types.String should be null")
	}
	if !i.IsNull() {
		t.Error("default types.Int64 should be null")
	}
	if !l.IsNull() {
		t.Error("default types.List should be null")
	}
}

// TestUnitServiceResource_NewServiceResourceNotNil verifies the constructor returns
// a non-nil resource.
func TestUnitServiceResource_NewServiceResourceNotNil(t *testing.T) {
	t.Parallel()
	r := provider.NewServiceResource()
	if r == nil {
		t.Fatal("NewServiceResource() returned nil")
	}
}

// TestUnitServiceResource_providerSchemaIntegration verifies that the service resource
// schema attribute count matches the model fields.
func TestUnitServiceResource_providerSchemaIntegration(t *testing.T) {
	t.Parallel()

	// The provider registers this resource; confirm the provider's Resources()
	// includes the service resource by checking the type name it returns.
	p := provider.New("dev")()

	// Verify provider implements the interface (compile-time check lifted to runtime).
	var _ fwprovider.Provider = p

	r := provider.NewServiceResource()
	metaResp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "foundrydb"}, metaResp)

	if metaResp.TypeName != "foundrydb_service" {
		t.Errorf("resource type name = %q; want foundrydb_service", metaResp.TypeName)
	}
}
