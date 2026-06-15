package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/anorph/terraform-provider-foundrydb/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// edgeDomainResponse builds a minimal API JSON body for an EdgeDomain.
func edgeDomainResponse(id, serviceID, domain, status string) map[string]interface{} {
	return map[string]interface{}{
		"id":           id,
		"service_id":   serviceID,
		"domain":       domain,
		"status":       status,
		"cname_target": "edge.foundrydb.com",
		"created_at":   "2026-01-01T00:00:00Z",
		"updated_at":   "2026-01-01T00:00:00Z",
	}
}

// configuredEdgeDomainResource returns an edgeDomainResource with a providerData
// configured against the provided httptest server URL.
func configuredEdgeDomainResource(t *testing.T, apiURL string) resource.Resource {
	t.Helper()
	r := provider.NewEdgeDomainResource()
	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewEdgeDomainResource() does not implement ResourceWithConfigure")
	}
	pd := provider.NewProviderDataForTest(apiURL, "admin", "admin")
	req := resource.ConfigureRequest{ProviderData: pd}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure failed: %v", resp.Diagnostics)
	}
	return r
}

// getEdgeDomainSchema returns the schema for the edge domain resource.
func getEdgeDomainSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() failed: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// edgeDomainStateModel mirrors edgeDomainResourceModel for state decoding in tests.
type edgeDomainStateModel struct {
	ID           types.String `tfsdk:"id"`
	AppServiceID types.String `tfsdk:"app_service_id"`
	Domain       types.String `tfsdk:"domain"`
	Status       types.String `tfsdk:"status"`
	CNAMETarget  types.String `tfsdk:"cname_target"`
}

// TestUnitEdgeDomainResource_Metadata verifies the resource type name.
func TestUnitEdgeDomainResource_Metadata(t *testing.T) {
	t.Parallel()
	r := provider.NewEdgeDomainResource()

	req := resource.MetadataRequest{ProviderTypeName: "foundrydb"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "foundrydb_edge_domain" {
		t.Errorf("TypeName = %q; want %q", resp.TypeName, "foundrydb_edge_domain")
	}
}

// TestUnitEdgeDomainResource_Schema_requiredAttributes verifies required attributes.
func TestUnitEdgeDomainResource_Schema_requiredAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewEdgeDomainResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	for _, key := range []string{"app_service_id", "domain"} {
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

// TestUnitEdgeDomainResource_Schema_computedAttributes verifies computed attributes.
func TestUnitEdgeDomainResource_Schema_computedAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewEdgeDomainResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"id", "status", "cname_target"} {
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

// TestUnitEdgeDomainResource_Schema_allExpectedFields verifies all expected attributes exist.
func TestUnitEdgeDomainResource_Schema_allExpectedFields(t *testing.T) {
	t.Parallel()
	r := provider.NewEdgeDomainResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, field := range []string{"id", "app_service_id", "domain", "status", "cname_target"} {
		if _, ok := resp.Schema.Attributes[field]; !ok {
			t.Errorf("expected attribute %q not found in edge domain resource schema", field)
		}
	}
}

// TestUnitEdgeDomainResource_Schema_markdownDescription verifies schema description.
func TestUnitEdgeDomainResource_Schema_markdownDescription(t *testing.T) {
	t.Parallel()
	r := provider.NewEdgeDomainResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Schema.MarkdownDescription == "" {
		t.Error("edge domain resource schema MarkdownDescription should not be empty")
	}
}

// TestUnitEdgeDomainResource_Configure_nilProviderData verifies Configure handles nil data.
func TestUnitEdgeDomainResource_Configure_nilProviderData(t *testing.T) {
	t.Parallel()
	r := provider.NewEdgeDomainResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewEdgeDomainResource() does not implement ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: nil}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with nil provider data should not produce errors; got: %v", resp.Diagnostics)
	}
}

// TestUnitEdgeDomainResource_Configure_wrongType verifies Configure errors on wrong type.
func TestUnitEdgeDomainResource_Configure_wrongType(t *testing.T) {
	t.Parallel()
	r := provider.NewEdgeDomainResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewEdgeDomainResource() does not implement ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: "not-a-providerData"}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure with wrong provider data type should produce an error")
	}
}

// TestUnitEdgeDomainCRUD_Create_success verifies Create POSTs to /domains and sets state.
func TestUnitEdgeDomainCRUD_Create_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-edge-001"
	const domainID = "domain-uuid-001"
	const domainName = "app.example.com"
	var createCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/domains"
		if r.Method == http.MethodPost && r.URL.Path == expected {
			createCalled.Store(true)
			body := edgeDomainResponse(domainID, appSvcID, domainName, "pending_verification")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredEdgeDomainResource(t, srv.URL)
	schema := getEdgeDomainSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"domain":         tftypes.NewValue(tftypes.String, domainName),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create returned errors: %v", resp.Diagnostics)
	}
	if !createCalled.Load() {
		t.Error("POST request was not sent to /domains")
	}

	var got edgeDomainStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", domainID, got.ID.ValueString())
	assertEq(t, "domain", domainName, got.Domain.ValueString())
	assertEq(t, "app_service_id", appSvcID, got.AppServiceID.ValueString())
	assertEq(t, "status", "pending_verification", got.Status.ValueString())
	assertEq(t, "cname_target", "edge.foundrydb.com", got.CNAMETarget.ValueString())
}

// TestUnitEdgeDomainCRUD_Create_apiError verifies Create surfaces API errors.
func TestUnitEdgeDomainCRUD_Create_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"domain already attached"}`))
	}))
	defer srv.Close()

	res := configuredEdgeDomainResource(t, srv.URL)
	schema := getEdgeDomainSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-edge-001"),
		"domain":         tftypes.NewValue(tftypes.String, "duplicate.example.com"),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Create should return a diagnostic error when the API returns 409")
	}
}

// TestUnitEdgeDomainCRUD_Read_success verifies Read finds the domain from the list.
func TestUnitEdgeDomainCRUD_Read_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-edge-002"
	const domainID = "domain-uuid-002"
	const domainName = "read.example.com"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/domains"
		if r.Method == http.MethodGet && r.URL.Path == expected {
			body := map[string]interface{}{
				"domains": []map[string]interface{}{
					edgeDomainResponse(domainID, appSvcID, domainName, "active"),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(body)
			w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredEdgeDomainResource(t, srv.URL)
	schema := getEdgeDomainSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, domainID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"domain":         tftypes.NewValue(tftypes.String, domainName),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}

	var got edgeDomainStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", domainID, got.ID.ValueString())
	assertEq(t, "domain", domainName, got.Domain.ValueString())
	assertEq(t, "status", "active", got.Status.ValueString())
}

// TestUnitEdgeDomainCRUD_Read_notFound verifies Read removes state when the domain is gone.
func TestUnitEdgeDomainCRUD_Read_notFound(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-edge-003"
	const domainID = "domain-gone-003"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return an empty domain list (domain was deleted externally).
		body := map[string]interface{}{"domains": []interface{}{}}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(body)
		w.Write(b)
	}))
	defer srv.Close()

	res := configuredEdgeDomainResource(t, srv.URL)
	schema := getEdgeDomainSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, domainID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"domain":         tftypes.NewValue(tftypes.String, "gone.example.com"),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read should not return errors when domain is not found; got: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be null when domain is not found (resource removed)")
	}
}

// TestUnitEdgeDomainCRUD_Read_apiError verifies Read propagates API errors.
func TestUnitEdgeDomainCRUD_Read_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	res := configuredEdgeDomainResource(t, srv.URL)
	schema := getEdgeDomainSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, "err-domain-001"),
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-edge-err"),
		"domain":         tftypes.NewValue(tftypes.String, "err.example.com"),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Read should return a diagnostic error when the API returns 500")
	}
}

// TestUnitEdgeDomainCRUD_Delete_success verifies Delete calls DELETE /domains/:id.
func TestUnitEdgeDomainCRUD_Delete_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-edge-004"
	const domainID = "domain-del-004"
	var deleted atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/domains/" + domainID
		if r.Method == http.MethodDelete && r.URL.Path == expected {
			deleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredEdgeDomainResource(t, srv.URL)
	schema := getEdgeDomainSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, domainID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"domain":         tftypes.NewValue(tftypes.String, "del.example.com"),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete returned errors: %v", resp.Diagnostics)
	}
	if !deleted.Load() {
		t.Error("DELETE request was not sent to the API")
	}
}

// TestUnitEdgeDomainCRUD_Delete_notFound verifies Delete treats 404 as success (idempotent).
func TestUnitEdgeDomainCRUD_Delete_notFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	res := configuredEdgeDomainResource(t, srv.URL)
	schema := getEdgeDomainSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, "already-gone-domain"),
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-edge-004"),
		"domain":         tftypes.NewValue(tftypes.String, "gone.example.com"),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete should treat 404 as success; got errors: %v", resp.Diagnostics)
	}
}

// TestUnitEdgeDomainResource_NewEdgeDomainResourceNotNil verifies the constructor returns non-nil.
func TestUnitEdgeDomainResource_NewEdgeDomainResourceNotNil(t *testing.T) {
	t.Parallel()
	r := provider.NewEdgeDomainResource()
	if r == nil {
		t.Fatal("NewEdgeDomainResource() returned nil")
	}
}
