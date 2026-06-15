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

// queueResponse builds a minimal API JSON body for a Queue.
func queueResponse(id, serviceID, name, status string) map[string]interface{} {
	return map[string]interface{}{
		"id":                         id,
		"service_id":                 serviceID,
		"name":                       name,
		"database_name":              "defaultdb",
		"visibility_timeout_seconds": 30,
		"max_attempts":               5,
		"dlq_enabled":                true,
		"status":                     status,
		"created_at":                 "2026-01-01T00:00:00Z",
		"updated_at":                 "2026-01-01T00:00:00Z",
	}
}

// configuredQueueResource returns a queueResource with a real *foundrydb.Client
// configured against the provided httptest server URL.
func configuredQueueResource(t *testing.T, apiURL string) resource.Resource {
	t.Helper()
	r := provider.NewQueueResource()
	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewQueueResource() does not implement ResourceWithConfigure")
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

// getQueueSchema returns the schema for the queue resource.
func getQueueSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() failed: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// buildQueueState constructs a tfsdk.State for the queue resource with the
// given attribute overrides.
func buildQueueState(t *testing.T, schema resourceschema.Schema, overrides map[string]tftypes.Value) tfsdk.State {
	t.Helper()
	return buildStateWithAttrs(t, schema, overrides)
}

// TestUnitQueueResource_Metadata verifies the resource type name is "foundrydb_queue".
func TestUnitQueueResource_Metadata(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	req := resource.MetadataRequest{ProviderTypeName: "foundrydb"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "foundrydb_queue" {
		t.Errorf("TypeName = %q; want %q", resp.TypeName, "foundrydb_queue")
	}
}

// TestUnitQueueResource_Schema_requiredAttributes verifies required attributes.
func TestUnitQueueResource_Schema_requiredAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	for _, key := range []string{"service_id", "name"} {
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

// TestUnitQueueResource_Schema_computedAttributes verifies computed-only attributes.
func TestUnitQueueResource_Schema_computedAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"id", "status", "database_name"} {
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

// TestUnitQueueResource_Schema_optionalComputedAttributes verifies optional+computed attributes.
func TestUnitQueueResource_Schema_optionalComputedAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"visibility_timeout_seconds", "max_attempts", "dlq_enabled"} {
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

// TestUnitQueueResource_Schema_forceNewAttributes verifies that creation-only attributes
// have RequiresReplace plan modifiers.
func TestUnitQueueResource_Schema_forceNewAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	// service_id and name must be ForceNew.
	for _, key := range []string{"service_id", "name"} {
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

	// visibility_timeout_seconds and max_attempts must also be ForceNew.
	for _, key := range []string{"visibility_timeout_seconds", "max_attempts"} {
		attr, ok := resp.Schema.Attributes[key]
		if !ok {
			t.Errorf("schema missing attribute %q", key)
			continue
		}
		int64Attr, ok := attr.(resourceschema.Int64Attribute)
		if !ok {
			t.Errorf("%q is not an Int64Attribute; got %T", key, attr)
			continue
		}
		if len(int64Attr.PlanModifiers) == 0 {
			t.Errorf("attribute %q should have plan modifiers (RequiresReplace)", key)
		}
	}

	// dlq_enabled must also be ForceNew.
	attr, ok := resp.Schema.Attributes["dlq_enabled"]
	if !ok {
		t.Fatal("schema missing dlq_enabled attribute")
	}
	boolAttr, ok := attr.(resourceschema.BoolAttribute)
	if !ok {
		t.Fatalf("dlq_enabled is not a BoolAttribute; got %T", attr)
	}
	if len(boolAttr.PlanModifiers) == 0 {
		t.Error("dlq_enabled should have plan modifiers (RequiresReplace)")
	}
}

// TestUnitQueueResource_Schema_markdownDescription verifies the schema has a description.
func TestUnitQueueResource_Schema_markdownDescription(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Schema.MarkdownDescription == "" {
		t.Error("queue resource schema MarkdownDescription should not be empty")
	}
}

// TestUnitQueueResource_Schema_allExpectedFields verifies all expected attributes exist.
func TestUnitQueueResource_Schema_allExpectedFields(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	expected := []string{
		"id", "service_id", "name", "visibility_timeout_seconds",
		"max_attempts", "dlq_enabled", "status", "database_name",
	}
	for _, field := range expected {
		if _, ok := resp.Schema.Attributes[field]; !ok {
			t.Errorf("expected attribute %q not found in queue resource schema", field)
		}
	}
}

// TestUnitQueueResource_Configure_nilProviderData verifies Configure does not panic.
func TestUnitQueueResource_Configure_nilProviderData(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewQueueResource() does not implement ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: nil}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with nil provider data should not produce errors; got: %v", resp.Diagnostics)
	}
}

// TestUnitQueueResource_Configure_wrongType verifies Configure errors on wrong type.
func TestUnitQueueResource_Configure_wrongType(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewQueueResource() does not implement ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: "not-a-client"}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure with wrong provider data type should produce an error")
	}
}

// TestUnitQueueCRUD_Read_success verifies Read populates state from the API.
func TestUnitQueueCRUD_Read_success(t *testing.T) {
	t.Parallel()

	const svcID = "pg-svc-001"
	const queueID = "q-uuid-001"
	const queueName = "task-queue"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/managed-services/" + svcID + "/queues/" + queueName
		if r.Method == http.MethodGet && r.URL.Path == expected {
			body := queueResponse(queueID, svcID, queueName, "Active")
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)
	state := buildQueueState(t, schema, map[string]tftypes.Value{
		"service_id": tftypes.NewValue(tftypes.String, svcID),
		"name":       tftypes.NewValue(tftypes.String, queueName),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}

	var got queueStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", queueID, got.ID.ValueString())
	assertEq(t, "service_id", svcID, got.ServiceID.ValueString())
	assertEq(t, "name", queueName, got.Name.ValueString())
	assertEq(t, "status", "Active", got.Status.ValueString())
	assertEq(t, "database_name", "defaultdb", got.DatabaseName.ValueString())

	if got.VisibilityTimeoutSeconds.ValueInt64() != 30 {
		t.Errorf("visibility_timeout_seconds = %d; want 30", got.VisibilityTimeoutSeconds.ValueInt64())
	}
	if got.MaxAttempts.ValueInt64() != 5 {
		t.Errorf("max_attempts = %d; want 5", got.MaxAttempts.ValueInt64())
	}
	if !got.DLQEnabled.ValueBool() {
		t.Error("dlq_enabled should be true")
	}
}

// TestUnitQueueCRUD_Read_notFound verifies Read removes resource from state on 404.
func TestUnitQueueCRUD_Read_notFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)
	state := buildQueueState(t, schema, map[string]tftypes.Value{
		"service_id": tftypes.NewValue(tftypes.String, "pg-svc-001"),
		"name":       tftypes.NewValue(tftypes.String, "gone-queue"),
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

// TestUnitQueueCRUD_Read_apiError verifies Read propagates API errors.
func TestUnitQueueCRUD_Read_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)
	state := buildQueueState(t, schema, map[string]tftypes.Value{
		"service_id": tftypes.NewValue(tftypes.String, "pg-svc-001"),
		"name":       tftypes.NewValue(tftypes.String, "err-queue"),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Read should return a diagnostic error when the API returns 500")
	}
}

// TestUnitQueueCRUD_Create_success verifies Create polls until Active and sets state.
func TestUnitQueueCRUD_Create_success(t *testing.T) {
	t.Parallel()

	const svcID = "pg-svc-002"
	const queueID = "q-uuid-002"
	const queueName = "events"

	var getPollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/managed-services/"+svcID+"/queues":
			body := queueResponse(queueID, svcID, queueName, "Provisioning")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(body))

		case r.Method == http.MethodGet && r.URL.Path == "/managed-services/"+svcID+"/queues/"+queueName:
			count := getPollCount.Add(1)
			status := "Provisioning"
			if count >= 2 {
				status = "Active"
			}
			body := queueResponse(queueID, svcID, queueName, status)
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)

	plan := buildQueueState(t, schema, map[string]tftypes.Value{
		"service_id": tftypes.NewValue(tftypes.String, svcID),
		"name":       tftypes.NewValue(tftypes.String, queueName),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create returned errors: %v", resp.Diagnostics)
	}

	var got queueStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "id", queueID, got.ID.ValueString())
	assertEq(t, "name", queueName, got.Name.ValueString())
	assertEq(t, "status", "Active", got.Status.ValueString())
	assertEq(t, "database_name", "defaultdb", got.DatabaseName.ValueString())
}

// TestUnitQueueCRUD_Create_failedStatus verifies Create errors when the queue reaches Failed status.
func TestUnitQueueCRUD_Create_failedStatus(t *testing.T) {
	t.Parallel()

	const svcID = "pg-svc-fail"
	const queueID = "q-uuid-fail"
	const queueName = "bad-queue"
	const errMsg = "schema creation failed"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/managed-services/"+svcID+"/queues":
			body := queueResponse(queueID, svcID, queueName, "Provisioning")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(body))

		case r.Method == http.MethodGet && r.URL.Path == "/managed-services/"+svcID+"/queues/"+queueName:
			body := queueResponse(queueID, svcID, queueName, "Failed")
			body["error_message"] = errMsg
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)

	plan := buildQueueState(t, schema, map[string]tftypes.Value{
		"service_id": tftypes.NewValue(tftypes.String, svcID),
		"name":       tftypes.NewValue(tftypes.String, queueName),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Create should return an error when the queue reaches Failed status")
	}
}

// TestUnitQueueCRUD_Create_apiError verifies Create surfaces POST errors.
func TestUnitQueueCRUD_Create_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"queue name already in use"}`))
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)

	plan := buildQueueState(t, schema, map[string]tftypes.Value{
		"service_id": tftypes.NewValue(tftypes.String, "pg-svc-999"),
		"name":       tftypes.NewValue(tftypes.String, "duplicate"),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Create should return a diagnostic error when the API returns 409")
	}
}

// TestUnitQueueCRUD_Delete_success verifies Delete calls DELETE, polls until gone, and succeeds.
func TestUnitQueueCRUD_Delete_success(t *testing.T) {
	t.Parallel()

	const svcID = "pg-svc-001"
	const queueID = "q-del-001"
	const queueName = "delete-me"
	var deleteCalled atomic.Bool
	var getPollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/managed-services/" + svcID + "/queues/" + queueName
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == path:
			deleteCalled.Store(true)
			body := queueResponse(queueID, svcID, queueName, "Deprovisioning")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			w.Write(jsonBody(body))

		case r.Method == http.MethodGet && r.URL.Path == path:
			count := getPollCount.Add(1)
			if count >= 2 {
				// Queue is gone.
				w.WriteHeader(http.StatusNotFound)
				return
			}
			body := queueResponse(queueID, svcID, queueName, "Deprovisioning")
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)
	state := buildQueueState(t, schema, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, queueID),
		"service_id": tftypes.NewValue(tftypes.String, svcID),
		"name":       tftypes.NewValue(tftypes.String, queueName),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete returned errors: %v", resp.Diagnostics)
	}
	if !deleteCalled.Load() {
		t.Error("DELETE request was not sent to the API")
	}
}

// TestUnitQueueCRUD_Delete_apiError verifies Delete surfaces DELETE errors.
func TestUnitQueueCRUD_Delete_apiError(t *testing.T) {
	t.Parallel()

	const svcID = "pg-svc-001"
	const queueName = "del-err-queue"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)
	state := buildQueueState(t, schema, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "q-del-err"),
		"service_id": tftypes.NewValue(tftypes.String, svcID),
		"name":       tftypes.NewValue(tftypes.String, queueName),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Delete should return a diagnostic error when the API returns 403")
	}
}

// TestUnitQueueCRUD_Create_withCustomSettings verifies Create sends custom settings to the API.
func TestUnitQueueCRUD_Create_withCustomSettings(t *testing.T) {
	t.Parallel()

	const svcID = "pg-svc-003"
	const queueID = "q-uuid-003"
	const queueName = "custom-queue"

	var receivedBody map[string]interface{}
	var getPollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/managed-services/"+svcID+"/queues":
			json.NewDecoder(r.Body).Decode(&receivedBody)
			body := queueResponse(queueID, svcID, queueName, "Provisioning")
			body["visibility_timeout_seconds"] = 60
			body["max_attempts"] = 3
			body["dlq_enabled"] = false
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(jsonBody(body))

		case r.Method == http.MethodGet && r.URL.Path == "/managed-services/"+svcID+"/queues/"+queueName:
			count := getPollCount.Add(1)
			status := "Provisioning"
			if count >= 2 {
				status = "Active"
			}
			body := queueResponse(queueID, svcID, queueName, status)
			body["visibility_timeout_seconds"] = 60
			body["max_attempts"] = 3
			body["dlq_enabled"] = false
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res := configuredQueueResource(t, srv.URL)
	schema := getQueueSchema(t, res)

	plan := buildQueueState(t, schema, map[string]tftypes.Value{
		"service_id":                 tftypes.NewValue(tftypes.String, svcID),
		"name":                       tftypes.NewValue(tftypes.String, queueName),
		"visibility_timeout_seconds": tftypes.NewValue(tftypes.Number, mustBigFloat("60")),
		"max_attempts":               tftypes.NewValue(tftypes.Number, mustBigFloat("3")),
		"dlq_enabled":                tftypes.NewValue(tftypes.Bool, false),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create with custom settings returned errors: %v", resp.Diagnostics)
	}

	// Verify the POST body included the settings.
	if vt, ok := receivedBody["visibility_timeout_seconds"].(float64); !ok || vt != 60 {
		t.Errorf("request visibility_timeout_seconds = %v; want 60", receivedBody["visibility_timeout_seconds"])
	}
	if ma, ok := receivedBody["max_attempts"].(float64); !ok || ma != 3 {
		t.Errorf("request max_attempts = %v; want 3", receivedBody["max_attempts"])
	}
	if dlq, ok := receivedBody["dlq_enabled"].(bool); !ok || dlq {
		t.Errorf("request dlq_enabled = %v; want false", receivedBody["dlq_enabled"])
	}

	var got queueStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "status", "Active", got.Status.ValueString())
	if got.VisibilityTimeoutSeconds.ValueInt64() != 60 {
		t.Errorf("visibility_timeout_seconds = %d; want 60", got.VisibilityTimeoutSeconds.ValueInt64())
	}
	if got.MaxAttempts.ValueInt64() != 3 {
		t.Errorf("max_attempts = %d; want 3", got.MaxAttempts.ValueInt64())
	}
	if got.DLQEnabled.ValueBool() {
		t.Error("dlq_enabled should be false")
	}
}

// TestUnitQueueResource_NewQueueResourceNotNil verifies the constructor returns non-nil.
func TestUnitQueueResource_NewQueueResourceNotNil(t *testing.T) {
	t.Parallel()
	r := provider.NewQueueResource()
	if r == nil {
		t.Fatal("NewQueueResource() returned nil")
	}
}

// queueStateModel mirrors queueResourceModel for state decoding in tests.
type queueStateModel struct {
	ID                       types.String `tfsdk:"id"`
	ServiceID                types.String `tfsdk:"service_id"`
	Name                     types.String `tfsdk:"name"`
	VisibilityTimeoutSeconds types.Int64  `tfsdk:"visibility_timeout_seconds"`
	MaxAttempts              types.Int64  `tfsdk:"max_attempts"`
	DLQEnabled               types.Bool   `tfsdk:"dlq_enabled"`
	Status                   types.String `tfsdk:"status"`
	DatabaseName             types.String `tfsdk:"database_name"`
}
