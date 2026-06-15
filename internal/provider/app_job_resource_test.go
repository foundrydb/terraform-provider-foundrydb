package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/anorph/terraform-provider-foundrydb/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// appJobResponse builds a minimal API JSON body for an AppJob.
func appJobResponse(id, serviceID, name string) map[string]interface{} {
	return map[string]interface{}{
		"id":                    id,
		"service_id":            serviceID,
		"name":                  name,
		"timezone":              "UTC",
		"enabled":               true,
		"max_retries":           0,
		"retry_backoff_seconds": 0,
		"max_runtime_seconds":   3600,
		"concurrency_cap":       1,
		"overlap_policy":        "allow",
		"created_at":            "2026-01-01T00:00:00Z",
		"updated_at":            "2026-01-01T00:00:00Z",
	}
}

// configuredAppJobResource returns an appJobResource with a real *foundrydb.Client
// configured against the provided httptest server URL.
func configuredAppJobResource(t *testing.T, apiURL string) resource.Resource {
	t.Helper()
	r := provider.NewAppJobResource()
	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewAppJobResource() does not implement ResourceWithConfigure")
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

// getAppJobSchema returns the schema for the app job resource.
func getAppJobSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() failed: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// buildAppJobState constructs a tfsdk.State for the app job resource with the
// given attribute overrides.
func buildAppJobState(t *testing.T, schema resourceschema.Schema, overrides map[string]tftypes.Value) tfsdk.State {
	t.Helper()
	return buildStateWithAttrs(t, schema, overrides)
}

// TestUnitAppJobResource_Metadata verifies the resource type name is "foundrydb_app_job".
func TestUnitAppJobResource_Metadata(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	req := resource.MetadataRequest{ProviderTypeName: "foundrydb"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "foundrydb_app_job" {
		t.Errorf("TypeName = %q; want %q", resp.TypeName, "foundrydb_app_job")
	}
}

// TestUnitAppJobResource_Schema_requiredAttributes verifies required attributes.
func TestUnitAppJobResource_Schema_requiredAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	for _, key := range []string{"app_service_id", "name"} {
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

// TestUnitAppJobResource_Schema_computedAttributes verifies computed-only attributes.
func TestUnitAppJobResource_Schema_computedAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"id"} {
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

// TestUnitAppJobResource_Schema_optionalComputedAttributes verifies optional+computed attributes.
func TestUnitAppJobResource_Schema_optionalComputedAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"timezone", "enabled", "command", "env", "max_retries", "retry_backoff_seconds", "max_runtime_seconds", "concurrency_cap"} {
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

// TestUnitAppJobResource_Schema_forceNewAttributes verifies ForceNew attributes.
func TestUnitAppJobResource_Schema_forceNewAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"app_service_id", "name"} {
		attr, ok := resp.Schema.Attributes[key]
		if !ok {
			t.Errorf("schema missing attribute %q", key)
			continue
		}
		strAttr, ok := attr.(resourceschema.StringAttribute)
		if !ok {
			t.Errorf("%q is not a StringAttribute; got %T", key, attr)
			continue
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("attribute %q should have plan modifiers (RequiresReplace)", key)
		}
	}
}

// TestUnitAppJobResource_Schema_markdownDescription verifies the schema has a description.
func TestUnitAppJobResource_Schema_markdownDescription(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Schema.MarkdownDescription == "" {
		t.Error("app job resource schema MarkdownDescription should not be empty")
	}
}

// TestUnitAppJobResource_Schema_allExpectedFields verifies all expected attributes exist.
func TestUnitAppJobResource_Schema_allExpectedFields(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	expected := []string{
		"id", "app_service_id", "name", "schedule_cron", "timezone", "enabled",
		"image_ref", "command", "env", "max_retries", "retry_backoff_seconds",
		"max_runtime_seconds", "concurrency_cap",
	}
	for _, field := range expected {
		if _, ok := resp.Schema.Attributes[field]; !ok {
			t.Errorf("expected attribute %q not found in app job resource schema", field)
		}
	}
}

// TestUnitAppJobResource_Configure_nilProviderData verifies Configure does not panic with nil data.
func TestUnitAppJobResource_Configure_nilProviderData(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewAppJobResource() does not implement ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: nil}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with nil provider data should not produce errors; got: %v", resp.Diagnostics)
	}
}

// TestUnitAppJobResource_Configure_wrongType verifies Configure errors on wrong type.
func TestUnitAppJobResource_Configure_wrongType(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewAppJobResource() does not implement ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: "not-a-client"}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure with wrong provider data type should produce an error")
	}
}

// TestUnitAppJobCRUD_Read_success verifies Read populates state from the API.
func TestUnitAppJobCRUD_Read_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-001"
	const jobID = "job-uuid-001"
	const jobName = "daily-report"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/jobs/" + jobID
		if r.Method == http.MethodGet && r.URL.Path == expected {
			body := appJobResponse(jobID, appSvcID, jobName)
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)
	state := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}

	var got appJobStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", jobID, got.ID.ValueString())
	assertEq(t, "name", jobName, got.Name.ValueString())
	assertEq(t, "app_service_id", appSvcID, got.AppServiceID.ValueString())
	assertEq(t, "timezone", "UTC", got.Timezone.ValueString())

	if !got.Enabled.ValueBool() {
		t.Error("enabled should be true")
	}
	if got.MaxRetries.ValueInt64() != 0 {
		t.Errorf("max_retries = %d; want 0", got.MaxRetries.ValueInt64())
	}
	if got.ConcurrencyCap.ValueInt64() != 1 {
		t.Errorf("concurrency_cap = %d; want 1", got.ConcurrencyCap.ValueInt64())
	}
	if got.MaxRuntimeSeconds.ValueInt64() != 3600 {
		t.Errorf("max_runtime_seconds = %d; want 3600", got.MaxRuntimeSeconds.ValueInt64())
	}
}

// TestUnitAppJobCRUD_Read_notFound verifies Read removes the resource from state on 404.
func TestUnitAppJobCRUD_Read_notFound(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-001"
	const jobID = "gone-job-001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)
	state := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned unexpected errors for 404: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be null after 404 response (resource removed)")
	}
}

// TestUnitAppJobCRUD_Read_apiError verifies Read propagates API errors.
func TestUnitAppJobCRUD_Read_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)
	state := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, "err-job-001"),
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-001"),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Read should return a diagnostic error when the API returns 500")
	}
}

// TestUnitAppJobCRUD_Create_success verifies Create calls POST and sets state.
func TestUnitAppJobCRUD_Create_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-002"
	const jobID = "job-uuid-002"
	const jobName = "nightly-cleanup"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/jobs"
		if r.Method == http.MethodPost && r.URL.Path == expected {
			body := appJobResponse(jobID, appSvcID, jobName)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)

	plan := buildAppJobState(t, schema, map[string]tftypes.Value{
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, jobName),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create returned errors: %v", resp.Diagnostics)
	}

	var got appJobStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", jobID, got.ID.ValueString())
	assertEq(t, "name", jobName, got.Name.ValueString())
	assertEq(t, "app_service_id", appSvcID, got.AppServiceID.ValueString())
}

// TestUnitAppJobCRUD_Create_withSchedule verifies Create sends schedule_cron.
func TestUnitAppJobCRUD_Create_withSchedule(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-003"
	const jobID = "job-uuid-003"
	const jobName = "scheduled-job"
	const cron = "0 2 * * *"

	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/jobs"
		if r.Method == http.MethodPost && r.URL.Path == expected {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			body := appJobResponse(jobID, appSvcID, jobName)
			body["schedule_cron"] = cron
			body["timezone"] = "Europe/Stockholm"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)

	plan := buildAppJobState(t, schema, map[string]tftypes.Value{
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, jobName),
		"schedule_cron":  tftypes.NewValue(tftypes.String, cron),
		"timezone":       tftypes.NewValue(tftypes.String, "Europe/Stockholm"),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create returned errors: %v", resp.Diagnostics)
	}

	if cronVal, _ := receivedBody["schedule_cron"].(string); cronVal != cron {
		t.Errorf("request schedule_cron = %q; want %q", cronVal, cron)
	}

	var got appJobStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "schedule_cron", cron, got.ScheduleCron.ValueString())
	assertEq(t, "timezone", "Europe/Stockholm", got.Timezone.ValueString())
}

// TestUnitAppJobCRUD_Create_apiError verifies Create surfaces API errors.
func TestUnitAppJobCRUD_Create_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"job name already in use"}`))
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)

	plan := buildAppJobState(t, schema, map[string]tftypes.Value{
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-999"),
		"name":           tftypes.NewValue(tftypes.String, "duplicate-job"),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Create should return a diagnostic error when the API returns 409")
	}
}

// TestUnitAppJobCRUD_Delete_success verifies Delete calls DELETE and succeeds.
func TestUnitAppJobCRUD_Delete_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-001"
	const jobID = "job-del-001"
	var deleted atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/jobs/" + jobID
		if r.Method == http.MethodDelete && r.URL.Path == expected {
			deleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)
	state := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, "my-job"),
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

// TestUnitAppJobCRUD_Delete_notFound verifies Delete treats 404 as success (idempotent).
func TestUnitAppJobCRUD_Delete_notFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)
	state := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, "already-gone-job"),
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-001"),
		"name":           tftypes.NewValue(tftypes.String, "gone-job"),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete should treat 404 as success; got errors: %v", resp.Diagnostics)
	}
}

// TestUnitAppJobCRUD_Update_success verifies Update sends PATCH with changed fields.
func TestUnitAppJobCRUD_Update_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-001"
	const jobID = "job-upd-001"
	var patchReceived atomic.Bool
	var patchBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/jobs/" + jobID
		if r.Method == http.MethodPatch && r.URL.Path == expected {
			patchReceived.Store(true)
			json.NewDecoder(r.Body).Decode(&patchBody)
			body := appJobResponse(jobID, appSvcID, "my-job")
			body["enabled"] = false
			body["max_retries"] = 3
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)

	plan := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, "my-job"),
		"enabled":        tftypes.NewValue(tftypes.Bool, false),
		"max_retries":    tftypes.NewValue(tftypes.Number, mustBigFloat("3")),
	})
	curState := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, "my-job"),
		"enabled":        tftypes.NewValue(tftypes.Bool, true),
		"max_retries":    tftypes.NewValue(tftypes.Number, mustBigFloat("0")),
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

	// Verify the PATCH body sent enabled=false.
	if en, _ := patchBody["enabled"].(bool); en {
		t.Error("PATCH body should have enabled=false")
	}

	var got appJobStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}
	if got.Enabled.ValueBool() {
		t.Error("state enabled should be false after update")
	}
	if got.MaxRetries.ValueInt64() != 3 {
		t.Errorf("state max_retries = %d; want 3", got.MaxRetries.ValueInt64())
	}
}

// TestUnitAppJobCRUD_Update_clearSchedule verifies Update sends clear_schedule when
// schedule_cron is removed from config.
func TestUnitAppJobCRUD_Update_clearSchedule(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-001"
	const jobID = "job-clr-cron"
	var patchBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			json.NewDecoder(r.Body).Decode(&patchBody)
			body := appJobResponse(jobID, appSvcID, "my-job")
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)

	// Plan has no schedule_cron (null); state had one.
	plan := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, "my-job"),
	})
	curState := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, "my-job"),
		"schedule_cron":  tftypes.NewValue(tftypes.String, "0 2 * * *"),
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

	if clearSched, _ := patchBody["clear_schedule"].(bool); !clearSched {
		t.Error("PATCH body should have clear_schedule=true when schedule_cron removed from config")
	}
}

// TestUnitAppJobCRUD_Update_clearImageRef verifies Update sends clear_image_ref when
// image_ref is removed from config.
func TestUnitAppJobCRUD_Update_clearImageRef(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-001"
	const jobID = "job-clr-img"
	var patchBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			json.NewDecoder(r.Body).Decode(&patchBody)
			body := appJobResponse(jobID, appSvcID, "my-job")
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppJobResource(t, srv.URL)
	schema := getAppJobSchema(t, res)

	// Plan has no image_ref (null); state had one.
	plan := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, "my-job"),
	})
	curState := buildAppJobState(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, jobID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"name":           tftypes.NewValue(tftypes.String, "my-job"),
		"image_ref":      tftypes.NewValue(tftypes.String, "registry.example.com/tools:v1"),
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

	if clearImg, _ := patchBody["clear_image_ref"].(bool); !clearImg {
		t.Error("PATCH body should have clear_image_ref=true when image_ref removed from config")
	}
}

// TestUnitAppJobResource_NewAppJobResourceNotNil verifies the constructor returns non-nil.
func TestUnitAppJobResource_NewAppJobResourceNotNil(t *testing.T) {
	t.Parallel()
	r := provider.NewAppJobResource()
	if r == nil {
		t.Fatal("NewAppJobResource() returned nil")
	}
}

// appJobStateModel mirrors appJobResourceModel for state decoding in tests.
type appJobStateModel struct {
	ID                  types.String `tfsdk:"id"`
	AppServiceID        types.String `tfsdk:"app_service_id"`
	Name                types.String `tfsdk:"name"`
	ScheduleCron        types.String `tfsdk:"schedule_cron"`
	Timezone            types.String `tfsdk:"timezone"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	ImageRef            types.String `tfsdk:"image_ref"`
	Command             types.List   `tfsdk:"command"`
	Env                 types.Map    `tfsdk:"env"`
	MaxRetries          types.Int64  `tfsdk:"max_retries"`
	RetryBackoffSeconds types.Int64  `tfsdk:"retry_backoff_seconds"`
	MaxRuntimeSeconds   types.Int64  `tfsdk:"max_runtime_seconds"`
	ConcurrencyCap      types.Int64  `tfsdk:"concurrency_cap"`
}
