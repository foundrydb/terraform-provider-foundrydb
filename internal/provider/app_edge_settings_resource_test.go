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

// edgeSettingsResponse builds a minimal API JSON body for EdgeSettings.
func edgeSettingsResponse(wafMode string, version int64) map[string]interface{} {
	return map[string]interface{}{
		"waf_mode":       wafMode,
		"config_version": version,
		"cache_rules":    []interface{}{},
	}
}

// edgeStatusResponse builds a minimal edge status envelope used by the Read path.
func edgeStatusResponse(wafMode string, version int64) map[string]interface{} {
	return map[string]interface{}{
		"edge_enabled":   true,
		"cname_target":   "edge.foundrydb.com",
		"config_version": version,
		"settings": map[string]interface{}{
			"waf_mode":       wafMode,
			"config_version": version,
			"cache_rules":    []interface{}{},
		},
	}
}

// configuredAppEdgeSettingsResource returns an appEdgeSettingsResource with a
// providerData configured against the provided httptest server URL.
func configuredAppEdgeSettingsResource(t *testing.T, apiURL string) resource.Resource {
	t.Helper()
	r := provider.NewAppEdgeSettingsResource()
	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewAppEdgeSettingsResource() does not implement ResourceWithConfigure")
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

// getAppEdgeSettingsSchema returns the schema for the app edge settings resource.
func getAppEdgeSettingsSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() failed: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// appEdgeSettingsStateModel mirrors appEdgeSettingsResourceModel for state
// decoding in tests.
type appEdgeSettingsStateModel struct {
	ID            types.String `tfsdk:"id"`
	AppServiceID  types.String `tfsdk:"app_service_id"`
	WAFMode       types.String `tfsdk:"waf_mode"`
	CacheRules    types.List   `tfsdk:"cache_rules"`
	RateLimit     types.List   `tfsdk:"rate_limit"`
	ConfigVersion types.Int64  `tfsdk:"config_version"`
}

// TestUnitAppEdgeSettingsResource_Metadata verifies the resource type name.
func TestUnitAppEdgeSettingsResource_Metadata(t *testing.T) {
	t.Parallel()
	r := provider.NewAppEdgeSettingsResource()

	req := resource.MetadataRequest{ProviderTypeName: "foundrydb"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "foundrydb_app_edge_settings" {
		t.Errorf("TypeName = %q; want %q", resp.TypeName, "foundrydb_app_edge_settings")
	}
}

// TestUnitAppEdgeSettingsResource_Schema_requiredAttributes verifies required attributes.
func TestUnitAppEdgeSettingsResource_Schema_requiredAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewAppEdgeSettingsResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["app_service_id"]
	if !ok {
		t.Fatal("schema missing required attribute app_service_id")
	}
	if !attr.IsRequired() {
		t.Error("attribute app_service_id should be Required")
	}
}

// TestUnitAppEdgeSettingsResource_Schema_computedAttributes verifies computed attributes.
func TestUnitAppEdgeSettingsResource_Schema_computedAttributes(t *testing.T) {
	t.Parallel()
	r := provider.NewAppEdgeSettingsResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, key := range []string{"id", "config_version"} {
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

// TestUnitAppEdgeSettingsResource_Schema_allExpectedFields verifies all expected attributes.
func TestUnitAppEdgeSettingsResource_Schema_allExpectedFields(t *testing.T) {
	t.Parallel()
	r := provider.NewAppEdgeSettingsResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	for _, field := range []string{"id", "app_service_id", "waf_mode", "cache_rules", "rate_limit", "config_version"} {
		if _, ok := resp.Schema.Attributes[field]; !ok {
			t.Errorf("expected attribute %q not found in app edge settings schema", field)
		}
	}
}

// TestUnitAppEdgeSettingsResource_Schema_markdownDescription verifies the schema description.
func TestUnitAppEdgeSettingsResource_Schema_markdownDescription(t *testing.T) {
	t.Parallel()
	r := provider.NewAppEdgeSettingsResource()

	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Schema.MarkdownDescription == "" {
		t.Error("app edge settings resource schema MarkdownDescription should not be empty")
	}
}

// TestUnitAppEdgeSettingsResource_Configure_nilProviderData verifies nil is handled cleanly.
func TestUnitAppEdgeSettingsResource_Configure_nilProviderData(t *testing.T) {
	t.Parallel()
	r := provider.NewAppEdgeSettingsResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewAppEdgeSettingsResource() does not implement ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: nil}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with nil provider data should not produce errors; got: %v", resp.Diagnostics)
	}
}

// TestUnitAppEdgeSettingsResource_Configure_wrongType verifies Configure errors on wrong type.
func TestUnitAppEdgeSettingsResource_Configure_wrongType(t *testing.T) {
	t.Parallel()
	r := provider.NewAppEdgeSettingsResource()

	configurable, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("NewAppEdgeSettingsResource() does not implement ResourceWithConfigure")
	}

	req := resource.ConfigureRequest{ProviderData: "not-a-providerData"}
	resp := &resource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure with wrong provider data type should produce an error")
	}
}

// TestUnitAppEdgeSettingsCRUD_Create_success verifies Create PUTs settings and sets state.
func TestUnitAppEdgeSettingsCRUD_Create_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-settings-001"
	var putCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/edge/settings"
		if r.Method == http.MethodPut && r.URL.Path == expected {
			putCalled.Store(true)
			body := edgeSettingsResponse("detect", 1)
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppEdgeSettingsResource(t, srv.URL)
	schema := getAppEdgeSettingsSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"waf_mode":       tftypes.NewValue(tftypes.String, "detect"),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create returned errors: %v", resp.Diagnostics)
	}
	if !putCalled.Load() {
		t.Error("PUT request was not sent to /edge/settings")
	}

	var got appEdgeSettingsStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "waf_mode", "detect", got.WAFMode.ValueString())
	assertEq(t, "id", appSvcID, got.ID.ValueString())
	if got.ConfigVersion.ValueInt64() != 1 {
		t.Errorf("config_version = %d; want 1", got.ConfigVersion.ValueInt64())
	}
}

// TestUnitAppEdgeSettingsCRUD_Create_apiError verifies Create surfaces API errors.
func TestUnitAppEdgeSettingsCRUD_Create_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid waf_mode"}`))
	}))
	defer srv.Close()

	res := configuredAppEdgeSettingsResource(t, srv.URL)
	schema := getAppEdgeSettingsSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-settings-bad"),
		"waf_mode":       tftypes.NewValue(tftypes.String, "invalid"),
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Create should return a diagnostic error when the API returns 400")
	}
}

// TestUnitAppEdgeSettingsCRUD_Read_success verifies Read populates state from edge status.
func TestUnitAppEdgeSettingsCRUD_Read_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-settings-002"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/edge"
		if r.Method == http.MethodGet && r.URL.Path == expected {
			body := edgeStatusResponse("off", 3)
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(body)
			w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppEdgeSettingsResource(t, srv.URL)
	schema := getAppEdgeSettingsSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, appSvcID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}

	var got appEdgeSettingsStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}

	assertEq(t, "waf_mode", "off", got.WAFMode.ValueString())
	if got.ConfigVersion.ValueInt64() != 3 {
		t.Errorf("config_version = %d; want 3", got.ConfigVersion.ValueInt64())
	}
}

// TestUnitAppEdgeSettingsCRUD_Read_apiError verifies Read propagates API errors.
func TestUnitAppEdgeSettingsCRUD_Read_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer srv.Close()

	res := configuredAppEdgeSettingsResource(t, srv.URL)
	schema := getAppEdgeSettingsSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, "app-svc-err"),
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-err"),
	})

	resp := &resource.ReadResponse{State: state}
	res.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Read should return a diagnostic error when the API returns 500")
	}
}

// TestUnitAppEdgeSettingsCRUD_Update_success verifies Update PUTs new settings.
func TestUnitAppEdgeSettingsCRUD_Update_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-settings-003"
	var putReceived atomic.Bool
	var putBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/edge/settings"
		if r.Method == http.MethodPut && r.URL.Path == expected {
			putReceived.Store(true)
			json.NewDecoder(r.Body).Decode(&putBody)
			body := edgeSettingsResponse("detect", 2)
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppEdgeSettingsResource(t, srv.URL)
	schema := getAppEdgeSettingsSchema(t, res)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, appSvcID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"waf_mode":       tftypes.NewValue(tftypes.String, "detect"),
	})
	curState := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, appSvcID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"waf_mode":       tftypes.NewValue(tftypes.String, "off"),
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
	if !putReceived.Load() {
		t.Error("PUT request was not sent to /edge/settings")
	}

	var got appEdgeSettingsStateModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("State.Get failed: %v", diags)
	}
	assertEq(t, "waf_mode after update", "detect", got.WAFMode.ValueString())
}

// TestUnitAppEdgeSettingsCRUD_Delete_success verifies Delete resets settings to defaults.
func TestUnitAppEdgeSettingsCRUD_Delete_success(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-settings-004"
	var putCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/app-services/" + appSvcID + "/edge/settings"
		if r.Method == http.MethodPut && r.URL.Path == expected {
			putCalled.Store(true)
			body := edgeSettingsResponse("off", 5)
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppEdgeSettingsResource(t, srv.URL)
	schema := getAppEdgeSettingsSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, appSvcID),
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"waf_mode":       tftypes.NewValue(tftypes.String, "detect"),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete returned errors: %v", resp.Diagnostics)
	}
	if !putCalled.Load() {
		t.Error("Delete should PUT reset settings but no request was sent")
	}
}

// TestUnitAppEdgeSettingsCRUD_Delete_apiError verifies Delete surfaces API errors.
func TestUnitAppEdgeSettingsCRUD_Delete_apiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	res := configuredAppEdgeSettingsResource(t, srv.URL)
	schema := getAppEdgeSettingsSchema(t, res)
	state := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, "app-svc-del-err"),
		"app_service_id": tftypes.NewValue(tftypes.String, "app-svc-del-err"),
		"waf_mode":       tftypes.NewValue(tftypes.String, "off"),
	})

	resp := &resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Delete should return a diagnostic error when the API returns 403")
	}
}

// TestUnitAppEdgeSettingsCRUD_Create_withCacheRules verifies cache_rules are sent correctly.
func TestUnitAppEdgeSettingsCRUD_Create_withCacheRules(t *testing.T) {
	t.Parallel()

	const appSvcID = "app-svc-settings-005"
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			body := map[string]interface{}{
				"waf_mode":       "off",
				"config_version": int64(1),
				"cache_rules": []map[string]interface{}{
					{"path_prefix": "/static", "ttl_seconds": 3600},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBody(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	res := configuredAppEdgeSettingsResource(t, srv.URL)
	schema := getAppEdgeSettingsSchema(t, res)

	cacheRulesList := tftypes.NewValue(
		tftypes.List{ElementType: tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"path_prefix": tftypes.String,
				"ttl_seconds": tftypes.Number,
			},
		}},
		[]tftypes.Value{
			tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"path_prefix": tftypes.String,
					"ttl_seconds": tftypes.Number,
				},
			}, map[string]tftypes.Value{
				"path_prefix": tftypes.NewValue(tftypes.String, "/static"),
				"ttl_seconds": tftypes.NewValue(tftypes.Number, mustBigFloat("3600")),
			}),
		},
	)

	plan := buildStateWithAttrs(t, schema, map[string]tftypes.Value{
		"app_service_id": tftypes.NewValue(tftypes.String, appSvcID),
		"cache_rules":    cacheRulesList,
	})
	initialState := buildNullState(t, schema)

	resp := &resource.CreateResponse{State: tfsdk.State(initialState)}
	res.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create with cache_rules returned errors: %v", resp.Diagnostics)
	}

	// Verify the PUT body included cache_rules.
	rules, _ := receivedBody["cache_rules"].([]interface{})
	if len(rules) != 1 {
		t.Errorf("PUT body cache_rules count = %d; want 1", len(rules))
		return
	}
	rule, _ := rules[0].(map[string]interface{})
	if rule["path_prefix"] != "/static" {
		t.Errorf("cache_rules[0].path_prefix = %v; want /static", rule["path_prefix"])
	}
}

// TestUnitAppEdgeSettingsResource_NewAppEdgeSettingsResourceNotNil verifies constructor.
func TestUnitAppEdgeSettingsResource_NewAppEdgeSettingsResourceNotNil(t *testing.T) {
	t.Parallel()
	r := provider.NewAppEdgeSettingsResource()
	if r == nil {
		t.Fatal("NewAppEdgeSettingsResource() returned nil")
	}
}
