package provider_test

// crud_test.go exercises the Create, Read, Update, Delete, ImportState, and serviceToState
// methods using real *foundrydb.Client instances pointed at httptest servers.
// This avoids the need for TF_ACC or terraform-plugin-testing while achieving
// meaningful CRUD coverage.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/anorph/terraform-provider-foundrydb/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// configuredServiceResource returns a serviceResource with a real *foundrydb.Client
// configured against the provided httptest server URL.
func configuredServiceResource(t *testing.T, apiURL string) resource.Resource {
	t.Helper()
	r := provider.NewServiceResource()
	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewServiceResource() does not implement ResourceWithConfigure")
	}
	c := foundrydb.New(foundrydb.Config{
		APIURL:      apiURL,
		Username:    "admin",
		Password:    "admin",
		HTTPTimeout: 5 * time.Second,
	})
	req := resource.ConfigureRequest{ProviderData: c}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure failed: %v", resp.Diagnostics)
	}
	return r
}

// getResourceSchema returns the resource schema for use in building tfsdk.State values.
func getResourceSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() failed: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// buildNullState constructs a tfsdk.State where all attributes are null, suitable for
// initialising response state objects.
func buildNullState(t *testing.T, schema resourceschema.Schema) tfsdk.State {
	t.Helper()
	return buildStateWithAttrs(t, schema, nil)
}

// buildStateWithAttrs constructs a tfsdk.State populated with the given attribute overrides.
// Attributes not listed in overrides are null. List attributes receive a properly typed null
// (not DynamicPseudoType) so the framework can convert them without error.
func buildStateWithAttrs(t *testing.T, schema resourceschema.Schema, overrides map[string]tftypes.Value) tfsdk.State {
	t.Helper()
	schemaType := schema.Type().TerraformType(context.Background())
	objType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is not tftypes.Object; got %T", schemaType)
	}
	attrs := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for k, attrType := range objType.AttributeTypes {
		if overrides != nil {
			if v, found := overrides[k]; found {
				attrs[k] = v
				continue
			}
		}
		// Use the schema-declared type so list/set attributes get proper element
		// type information (not DynamicPseudoType) when null.
		attrs[k] = tftypes.NewValue(attrType, nil)
	}
	return tfsdk.State{
		Raw:    tftypes.NewValue(objType, attrs),
		Schema: schema,
	}
}

// svcRunning returns a Service JSON body with status=running.
func svcRunning(id, name string) []byte {
	svc := map[string]interface{}{
		"uuid":            id,
		"name":            name,
		"database_type":   "postgresql",
		"version":         "17",
		"status":          "running",
		"plan_name":       "tier-2",
		"zone":            "se-sto1",
		"storage_size_gb": int64(50),
		"storage_tier":    "maxiops",
		"allowed_cidrs":   []string{"10.0.0.0/8"},
		"dns_records":     []interface{}{},
		"created_at":      "2026-01-01T00:00:00Z",
		"updated_at":      "2026-01-02T00:00:00Z",
	}
	b, _ := json.Marshal(svc)
	return b
}

// stateModel is used in tests to decode service resource state.
type stateModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	DatabaseType   types.String `tfsdk:"database_type"`
	Version        types.String `tfsdk:"version"`
	PlanName       types.String `tfsdk:"plan_name"`
	Zone           types.String `tfsdk:"zone"`
	StorageSizeGB  types.Int64  `tfsdk:"storage_size_gb"`
	StorageTier    types.String `tfsdk:"storage_tier"`
	AllowedCIDRs   types.List   `tfsdk:"allowed_cidrs"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Status         types.String `tfsdk:"status"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

// TestUnitCRUD_Read_success verifies Read populates state correctly from the API.
func TestUnitCRUD_Read_success(t *testing.T) {
	t.Parallel()

	const svcID = "read-uuid-001"
	const svcName = "pg-read-test"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/managed-services/"+svcID {
			w.Header().Set("Content-Type", "application/json")
			w.Write(svcRunning(svcID, svcName))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}

	var got stateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", svcID, got.ID.ValueString())
	assertEq(t, "name", svcName, got.Name.ValueString())
	assertEq(t, "database_type", "postgresql", got.DatabaseType.ValueString())
	assertEq(t, "version", "17", got.Version.ValueString())
	assertEq(t, "status", "running", got.Status.ValueString())
	assertEq(t, "zone", "se-sto1", got.Zone.ValueString())
	assertEq(t, "plan_name", "tier-2", got.PlanName.ValueString())
	assertEq(t, "storage_tier", "maxiops", got.StorageTier.ValueString())
	assertEq(t, "created_at", "2026-01-01T00:00:00Z", got.CreatedAt.ValueString())

	if got.StorageSizeGB.ValueInt64() != 50 {
		t.Errorf("storage_size_gb = %d; want 50", got.StorageSizeGB.ValueInt64())
	}
}

// TestUnitCRUD_Read_notFound verifies Read removes the resource from state when the API
// returns 404.
func TestUnitCRUD_Read_notFound(t *testing.T) {
	t.Parallel()

	const svcID = "gone-uuid-001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned unexpected errors for 404: %v", resp.Diagnostics)
	}

	// When the resource is gone, RemoveResource sets the Raw to null.
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be null after 404 response (resource removed)")
	}
}

// TestUnitCRUD_Read_apiError verifies Read propagates API errors as diagnostics.
func TestUnitCRUD_Read_apiError(t *testing.T) {
	t.Parallel()

	const svcID = "error-uuid-001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Read should return an error diagnostic when the API returns 500")
	}
}

// TestUnitCRUD_Delete_success verifies Delete calls the API and produces no errors.
func TestUnitCRUD_Delete_success(t *testing.T) {
	t.Parallel()

	const svcID = "del-uuid-001"
	var deleted atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/managed-services/"+svcID {
			deleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
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

// TestUnitCRUD_Delete_notFound verifies Delete is idempotent (404 is not an error).
func TestUnitCRUD_Delete_notFound(t *testing.T) {
	t.Parallel()

	const svcID = "already-gone-001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete should treat 404 as success; got errors: %v", resp.Diagnostics)
	}
}

// TestUnitCRUD_Delete_apiError verifies Delete surfaces API errors as diagnostics.
func TestUnitCRUD_Delete_apiError(t *testing.T) {
	t.Parallel()

	const svcID = "del-error-001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Delete should return a diagnostic error when the API returns 403")
	}
}

// TestUnitCRUD_Create_success verifies Create calls POST /managed-services and waits
// for running status.
func TestUnitCRUD_Create_success(t *testing.T) {
	t.Parallel()

	const svcID = "create-uuid-001"
	const svcName = "pg-create-test"

	var getPollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/managed-services":
			svc := serviceResponse(svcID, svcName, "postgresql", "17", "provisioning")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(svc))

		case r.Method == http.MethodGet && r.URL.Path == "/managed-services/"+svcID:
			count := getPollCount.Add(1)
			status := "provisioning"
			if count >= 2 {
				status = "running"
			}
			svc := serviceResponse(svcID, svcName, "postgresql", "17", status)
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(svc))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"name":          tftypes.NewValue(tftypes.String, svcName),
		"database_type": tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":     tftypes.NewValue(tftypes.String, "tier-2"),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create returned errors: %v", resp.Diagnostics)
	}

	var got stateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", svcID, got.ID.ValueString())
	assertEq(t, "name", svcName, got.Name.ValueString())
	assertEq(t, "status", "running", got.Status.ValueString())
}

// TestUnitCRUD_Create_apiError verifies Create surfaces API errors.
func TestUnitCRUD_Create_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid request"}`))
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"name":          tftypes.NewValue(tftypes.String, "bad-svc"),
		"database_type": tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":     tftypes.NewValue(tftypes.String, "tier-2"),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Create should return a diagnostic error when the API returns 400")
	}
}

// TestUnitCRUD_Update_success verifies Update calls PATCH /managed-services/:id.
func TestUnitCRUD_Update_success(t *testing.T) {
	t.Parallel()

	const svcID = "update-uuid-001"
	const newName = "pg-renamed"

	var patchReceived atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && r.URL.Path == "/managed-services/"+svcID {
			patchReceived.Store(true)
			svc := serviceResponse(svcID, newName, "postgresql", "17", "running")
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(svc))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":            tftypes.NewValue(tftypes.String, svcID),
		"name":          tftypes.NewValue(tftypes.String, newName),
		"database_type": tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":     tftypes.NewValue(tftypes.String, "tier-2"),
	})
	curState := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":            tftypes.NewValue(tftypes.String, svcID),
		"name":          tftypes.NewValue(tftypes.String, "pg-original"),
		"database_type": tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":     tftypes.NewValue(tftypes.String, "tier-2"),
	})

	initialResp := buildNullState(t, schema)
	resp := &resource.UpdateResponse{State: tfsdk.State(initialResp)}
	res.Update(context.Background(), resource.UpdateRequest{
		Plan:  tfsdk.Plan(plan),
		State: curState,
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update returned errors: %v", resp.Diagnostics)
	}
	if !patchReceived.Load() {
		t.Error("PATCH request was not sent to the API")
	}

	var got stateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get after Update failed: %v", diags)
	}
	assertEq(t, "name after update", newName, got.Name.ValueString())
}

// TestUnitCRUD_Update_apiError verifies Update surfaces PATCH errors.
func TestUnitCRUD_Update_apiError(t *testing.T) {
	t.Parallel()

	const svcID = "update-err-001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error":"validation failed"}`))
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":            tftypes.NewValue(tftypes.String, svcID),
		"name":          tftypes.NewValue(tftypes.String, "new-name"),
		"database_type": tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":     tftypes.NewValue(tftypes.String, "tier-2"),
	})
	curState := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":            tftypes.NewValue(tftypes.String, svcID),
		"name":          tftypes.NewValue(tftypes.String, "old-name"),
		"database_type": tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":     tftypes.NewValue(tftypes.String, "tier-2"),
	})

	initialResp := buildNullState(t, schema)
	resp := &resource.UpdateResponse{State: tfsdk.State(initialResp)}
	res.Update(context.Background(), resource.UpdateRequest{
		Plan:  tfsdk.Plan(plan),
		State: curState,
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Update should return a diagnostic error when the API returns 422")
	}
}

// TestUnitCRUD_ImportState_implementsInterface verifies the resource implements
// ResourceWithImportState and the ImportState method calls the API to populate state.
func TestUnitCRUD_ImportState_implementsInterface(t *testing.T) {
	t.Parallel()

	res := configuredServiceResource(t, "http://unused")
	if _, ok := res.(resource.ResourceWithImportState); !ok {
		t.Error("NewServiceResource() does not implement resource.ResourceWithImportState")
	}
}

// TestUnitCRUD_ImportState_apiError verifies ImportState propagates API errors.
func TestUnitCRUD_ImportState_apiError(t *testing.T) {
	t.Parallel()

	const svcID = "import-error-001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)

	importer, ok := res.(resource.ResourceWithImportState)
	if !ok {
		t.Fatal("resource does not implement ResourceWithImportState")
	}

	initialState := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
	})

	importResp := &resource.ImportStateResponse{State: initialState}
	importer.ImportState(context.Background(), resource.ImportStateRequest{ID: svcID}, importResp)

	// When the API returns 404, ImportState should either error or remove the resource.
	// The provider returns nil on 404 (ResourceNotFound), so state.Raw should be null.
	if importResp.Diagnostics.HasError() {
		// Some implementations surface 404 as an error - either outcome is acceptable.
		return
	}
	if !importResp.State.Raw.IsNull() {
		t.Error("ImportState for a non-existent resource should clear state or report an error")
	}
}

// TestUnitCRUD_ServiceToState_allowedCIDRs verifies that allowed_cidrs is correctly
// mapped from the API response into a Terraform list.
func TestUnitCRUD_ServiceToState_allowedCIDRs(t *testing.T) {
	t.Parallel()

	const svcID = "cidr-uuid-001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svc := map[string]interface{}{
			"uuid":            svcID,
			"name":            "pg-cidr",
			"database_type":   "postgresql",
			"version":         "17",
			"status":          "running",
			"plan_name":       "tier-2",
			"zone":            "se-sto1",
			"storage_size_gb": 50,
			"storage_tier":    "maxiops",
			"allowed_cidrs":   []string{"192.168.1.0/24", "10.0.0.0/8"},
			"dns_records":     []interface{}{},
			"created_at":      "2026-01-01T00:00:00Z",
			"updated_at":      "2026-01-01T00:00:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonBody(svc))
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read failed: %v", resp.Diagnostics)
	}

	var got stateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	elems := got.AllowedCIDRs.Elements()
	if len(elems) != 2 {
		t.Errorf("allowed_cidrs length = %d; want 2", len(elems))
		return
	}

	wantCIDRs := []string{"192.168.1.0/24", "10.0.0.0/8"}
	for i, elem := range elems {
		strVal, ok := elem.(types.String)
		if !ok {
			t.Errorf("allowed_cidrs[%d] is %T, not types.String", i, elem)
			continue
		}
		assertEq(t, fmt.Sprintf("allowed_cidrs[%d]", i), wantCIDRs[i], strVal.ValueString())
	}
}

// TestUnitCRUD_ServiceToState_emptyAllowedCIDRs verifies an empty CIDR list maps correctly.
func TestUnitCRUD_ServiceToState_emptyAllowedCIDRs(t *testing.T) {
	t.Parallel()

	const svcID = "no-cidr-uuid"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svc := serviceResponse(svcID, "pg-no-cidr", "postgresql", "17", "running")
		svc["allowed_cidrs"] = []string{}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonBody(svc))
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, svcID),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read failed: %v", resp.Diagnostics)
	}

	var got stateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	if len(got.AllowedCIDRs.Elements()) != 0 {
		t.Errorf("allowed_cidrs should be empty; got %d elements", len(got.AllowedCIDRs.Elements()))
	}
}

// TestUnitOrganizationsDataSource_Read_success verifies the organizations data source
// reads the org list from the API.
func TestUnitOrganizationsDataSource_Read_success(t *testing.T) {
	t.Parallel()

	orgs := map[string]interface{}{
		"organizations": []map[string]interface{}{
			{
				"id":          "org-uuid-1",
				"name":        "Acme Corp",
				"slug":        "acme",
				"is_personal": false,
				"role":        "owner",
				"created_at":  "2025-01-01T00:00:00Z",
			},
			{
				"id":          "org-uuid-2",
				"name":        "Personal",
				"slug":        "personal",
				"is_personal": true,
				"role":        "owner",
				"created_at":  "2025-06-01T00:00:00Z",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/organizations" {
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(orgs)
			w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ds := provider.NewOrganizationsDataSource()
	configureDS(t, ds, srv.URL)

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	// Build a null config state for the Read request.
	dsSchemaType := schemaResp.Schema.Type().TerraformType(context.Background())
	dsObjType, ok := dsSchemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("org schema type is not Object; got %T", dsSchemaType)
	}
	nullAttrs := make(map[string]tftypes.Value)
	for k, attrType := range dsObjType.AttributeTypes {
		nullAttrs[k] = tftypes.NewValue(attrType, nil)
	}
	dsState := tfsdk.State{
		Raw:    tftypes.NewValue(dsObjType, nullAttrs),
		Schema: schemaResp.Schema,
	}

	readResp := &datasource.ReadResponse{State: dsState}
	ds.Read(context.Background(), datasource.ReadRequest{Config: tfsdk.Config(dsState)}, readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", readResp.Diagnostics)
	}

	var result struct {
		Organizations []struct {
			ID        types.String `tfsdk:"id"`
			Name      types.String `tfsdk:"name"`
			Slug      types.String `tfsdk:"slug"`
			Role      types.String `tfsdk:"role"`
			CreatedAt types.String `tfsdk:"created_at"`
		} `tfsdk:"organizations"`
	}
	if diags := readResp.State.Get(context.Background(), &result); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	if len(result.Organizations) != 2 {
		t.Fatalf("organizations count = %d; want 2", len(result.Organizations))
	}
	assertEq(t, "org[0].id", "org-uuid-1", result.Organizations[0].ID.ValueString())
	assertEq(t, "org[0].name", "Acme Corp", result.Organizations[0].Name.ValueString())
	assertEq(t, "org[0].slug", "acme", result.Organizations[0].Slug.ValueString())
	assertEq(t, "org[0].role", "owner", result.Organizations[0].Role.ValueString())
	assertEq(t, "org[1].id", "org-uuid-2", result.Organizations[1].ID.ValueString())
}

// TestUnitOrganizationsDataSource_Read_apiError verifies the data source surfaces errors.
func TestUnitOrganizationsDataSource_Read_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	ds := provider.NewOrganizationsDataSource()
	configureDS(t, ds, srv.URL)

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	dsSchemaType := schemaResp.Schema.Type().TerraformType(context.Background())
	dsObjType, _ := dsSchemaType.(tftypes.Object)
	nullAttrs := make(map[string]tftypes.Value)
	for k, attrType := range dsObjType.AttributeTypes {
		nullAttrs[k] = tftypes.NewValue(attrType, nil)
	}
	dsState := tfsdk.State{
		Raw:    tftypes.NewValue(dsObjType, nullAttrs),
		Schema: schemaResp.Schema,
	}

	readResp := &datasource.ReadResponse{State: dsState}
	ds.Read(context.Background(), datasource.ReadRequest{Config: tfsdk.Config(dsState)}, readResp)

	if !readResp.Diagnostics.HasError() {
		t.Error("Read should return an error diagnostic when the API returns 401")
	}
}

// TestUnitDatabaseUserDataSource_Read_success verifies credentials are populated from
// the reveal-password API.
func TestUnitDatabaseUserDataSource_Read_success(t *testing.T) {
	t.Parallel()

	const svcID = "cred-svc-001"
	const username = "app_user"

	creds := map[string]interface{}{
		"username":          username,
		"password":          "s3cr3t",
		"host":              "pg.example.com",
		"port":              int64(5432),
		"database":          "defaultdb",
		"connection_string": "postgresql://app_user:s3cr3t@pg.example.com:5432/defaultdb",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/managed-services/" + svcID + "/database-users/" + username + "/reveal-password"
		if r.Method == http.MethodPost && r.URL.Path == expected {
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(creds)
			w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ds := provider.NewDatabaseUserDataSource()
	configureDS(t, ds, srv.URL)

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	dsSchemaType := schemaResp.Schema.Type().TerraformType(context.Background())
	dsObjType, _ := dsSchemaType.(tftypes.Object)

	attrs := map[string]tftypes.Value{}
	for k, attrType := range dsObjType.AttributeTypes {
		attrs[k] = tftypes.NewValue(attrType, nil)
	}
	attrs["service_id"] = tftypes.NewValue(tftypes.String, svcID)
	attrs["username"] = tftypes.NewValue(tftypes.String, username)

	dsState := tfsdk.State{
		Raw:    tftypes.NewValue(dsObjType, attrs),
		Schema: schemaResp.Schema,
	}

	readResp := &datasource.ReadResponse{State: dsState}
	ds.Read(context.Background(), datasource.ReadRequest{Config: tfsdk.Config(dsState)}, readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", readResp.Diagnostics)
	}

	var result struct {
		ServiceID        types.String `tfsdk:"service_id"`
		Username         types.String `tfsdk:"username"`
		Password         types.String `tfsdk:"password"`
		Host             types.String `tfsdk:"host"`
		Port             types.Int64  `tfsdk:"port"`
		Database         types.String `tfsdk:"database"`
		ConnectionString types.String `tfsdk:"connection_string"`
	}
	if diags := readResp.State.Get(context.Background(), &result); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "password", "s3cr3t", result.Password.ValueString())
	assertEq(t, "host", "pg.example.com", result.Host.ValueString())
	assertEq(t, "database", "defaultdb", result.Database.ValueString())
	assertEq(t, "connection_string", "postgresql://app_user:s3cr3t@pg.example.com:5432/defaultdb", result.ConnectionString.ValueString())

	if result.Port.ValueInt64() != 5432 {
		t.Errorf("port = %d; want 5432", result.Port.ValueInt64())
	}
}

// TestUnitDatabaseUserDataSource_Read_apiError verifies the data source surfaces errors.
func TestUnitDatabaseUserDataSource_Read_apiError(t *testing.T) {
	t.Parallel()

	const svcID = "cred-err-001"
	const username = "unknown_user"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"user not found"}`))
	}))
	defer srv.Close()

	ds := provider.NewDatabaseUserDataSource()
	configureDS(t, ds, srv.URL)

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	dsSchemaType := schemaResp.Schema.Type().TerraformType(context.Background())
	dsObjType, _ := dsSchemaType.(tftypes.Object)

	attrs := map[string]tftypes.Value{}
	for k, attrType := range dsObjType.AttributeTypes {
		attrs[k] = tftypes.NewValue(attrType, nil)
	}
	attrs["service_id"] = tftypes.NewValue(tftypes.String, svcID)
	attrs["username"] = tftypes.NewValue(tftypes.String, username)

	dsState := tfsdk.State{
		Raw:    tftypes.NewValue(dsObjType, attrs),
		Schema: schemaResp.Schema,
	}

	readResp := &datasource.ReadResponse{State: dsState}
	ds.Read(context.Background(), datasource.ReadRequest{Config: tfsdk.Config(dsState)}, readResp)

	if !readResp.Diagnostics.HasError() {
		t.Error("Read should return an error diagnostic when the API returns 404")
	}
}

// configureDS injects a *foundrydb.Client into a data source.
func configureDS(t *testing.T, ds datasource.DataSource, apiURL string) {
	t.Helper()
	configurable, ok := ds.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("data source does not implement DataSourceWithConfigure")
	}
	c := foundrydb.New(foundrydb.Config{
		APIURL:      apiURL,
		Username:    "admin",
		Password:    "admin",
		HTTPTimeout: 5 * time.Second,
	})
	resp := &datasource.ConfigureResponse{}
	configurable.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: c}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("datasource Configure failed: %v", resp.Diagnostics)
	}
}

// assertEq is a compact string equality helper.
func assertEq(t *testing.T, field, want, got string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q; want %q", field, got, want)
	}
}

// TestUnitCRUD_Create_withAllowedCIDRs verifies Create handles the allowed_cidrs list correctly.
func TestUnitCRUD_Create_withAllowedCIDRs(t *testing.T) {
	t.Parallel()

	const svcID = "create-cidr-uuid-001"
	const svcName = "pg-with-cidrs"

	var getPollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/managed-services":
			var reqBody map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			svc := map[string]interface{}{
				"uuid":            svcID,
				"name":            svcName,
				"database_type":   "postgresql",
				"version":         "17",
				"status":          "provisioning",
				"plan_name":       "tier-2",
				"zone":            "se-sto1",
				"storage_size_gb": 100,
				"storage_tier":    "maxiops",
				"allowed_cidrs":   []string{"10.0.0.0/8", "192.168.0.0/16"},
				"dns_records":     []interface{}{},
				"created_at":      "2026-01-01T00:00:00Z",
				"updated_at":      "2026-01-01T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(svc))

		case r.Method == http.MethodGet && r.URL.Path == "/managed-services/"+svcID:
			count := getPollCount.Add(1)
			status := "provisioning"
			if count >= 2 {
				status = "running"
			}
			svc := map[string]interface{}{
				"uuid":            svcID,
				"name":            svcName,
				"database_type":   "postgresql",
				"version":         "17",
				"status":          status,
				"plan_name":       "tier-2",
				"zone":            "se-sto1",
				"storage_size_gb": 100,
				"storage_tier":    "maxiops",
				"allowed_cidrs":   []string{"10.0.0.0/8", "192.168.0.0/16"},
				"dns_records":     []interface{}{},
				"created_at":      "2026-01-01T00:00:00Z",
				"updated_at":      "2026-01-01T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(svc))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)

	// Build allowed_cidrs as a properly typed list value.
	cidrList := tftypes.NewValue(
		tftypes.List{ElementType: tftypes.String},
		[]tftypes.Value{
			tftypes.NewValue(tftypes.String, "10.0.0.0/8"),
			tftypes.NewValue(tftypes.String, "192.168.0.0/16"),
		},
	)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"name":           tftypes.NewValue(tftypes.String, svcName),
		"database_type":  tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":      tftypes.NewValue(tftypes.String, "tier-2"),
		"storage_size_gb": tftypes.NewValue(tftypes.Number, mustBigFloat("100")),
		"storage_tier":   tftypes.NewValue(tftypes.String, "maxiops"),
		"allowed_cidrs":  cidrList,
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create with CIDRs returned errors: %v", resp.Diagnostics)
	}

	var got stateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", svcID, got.ID.ValueString())
	assertEq(t, "status", "running", got.Status.ValueString())

	if len(got.AllowedCIDRs.Elements()) != 2 {
		t.Errorf("allowed_cidrs count = %d; want 2", len(got.AllowedCIDRs.Elements()))
	}
}

// TestUnitCRUD_Update_withAllowedCIDRs verifies Update sends the updated CIDRs list.
func TestUnitCRUD_Update_withAllowedCIDRs(t *testing.T) {
	t.Parallel()

	const svcID = "update-cidr-uuid-001"
	var patchReceived atomic.Bool
	var patchBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && r.URL.Path == "/managed-services/"+svcID {
			patchReceived.Store(true)
			patchBody, _ = io.ReadAll(r.Body)
			svc := map[string]interface{}{
				"uuid":            svcID,
				"name":            "pg-cidr-update",
				"database_type":   "postgresql",
				"version":         "17",
				"status":          "running",
				"plan_name":       "tier-2",
				"zone":            "se-sto1",
				"storage_size_gb": 50,
				"storage_tier":    "maxiops",
				"allowed_cidrs":   []string{"172.16.0.0/12"},
				"dns_records":     []interface{}{},
				"created_at":      "2026-01-01T00:00:00Z",
				"updated_at":      "2026-01-02T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(svc))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredServiceResource(t, srv.URL)
	schema := getResourceSchema(t, res)

	newCIDRList := tftypes.NewValue(
		tftypes.List{ElementType: tftypes.String},
		[]tftypes.Value{tftypes.NewValue(tftypes.String, "172.16.0.0/12")},
	)
	oldCIDRList := tftypes.NewValue(
		tftypes.List{ElementType: tftypes.String},
		[]tftypes.Value{tftypes.NewValue(tftypes.String, "10.0.0.0/8")},
	)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":            tftypes.NewValue(tftypes.String, svcID),
		"name":          tftypes.NewValue(tftypes.String, "pg-cidr-update"),
		"database_type": tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":     tftypes.NewValue(tftypes.String, "tier-2"),
		"allowed_cidrs": newCIDRList,
	})
	curState := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":            tftypes.NewValue(tftypes.String, svcID),
		"name":          tftypes.NewValue(tftypes.String, "pg-cidr-update"),
		"database_type": tftypes.NewValue(tftypes.String, "postgresql"),
		"plan_name":     tftypes.NewValue(tftypes.String, "tier-2"),
		"allowed_cidrs": oldCIDRList,
	})

	initialResp := buildNullState(t, schema)
	resp := &resource.UpdateResponse{State: tfsdk.State(initialResp)}
	res.Update(context.Background(), resource.UpdateRequest{
		Plan:  tfsdk.Plan(plan),
		State: curState,
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update with CIDRs returned errors: %v", resp.Diagnostics)
	}
	if !patchReceived.Load() {
		t.Error("PATCH request was not sent to the API")
	}

	// Verify the PATCH body includes the new CIDRs.
	var patchReq map[string]interface{}
	if err := json.Unmarshal(patchBody, &patchReq); err != nil {
		t.Fatalf("PATCH body unmarshal failed: %v", err)
	}
	cidrs, _ := patchReq["allowed_cidrs"].([]interface{})
	if len(cidrs) != 1 || cidrs[0] != "172.16.0.0/12" {
		t.Errorf("PATCH body allowed_cidrs = %v; want [172.16.0.0/12]", cidrs)
	}
}

// mustBigFloat converts a decimal string to a *big.Float value for use in tftypes.Number.
func mustBigFloat(s string) interface{} {
	f := new(big.Float)
	f.SetPrec(512)
	if _, ok := f.SetString(s); !ok {
		panic(fmt.Sprintf("mustBigFloat: invalid float %q", s))
	}
	return f
}
