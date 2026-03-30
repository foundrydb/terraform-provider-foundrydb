package provider_test

import (
	"context"
	"testing"

	"github.com/anorph/terraform-provider-foundrydb/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// TestUnitDatabaseUserDataSource_Metadata verifies the data source type name.
func TestUnitDatabaseUserDataSource_Metadata(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.MetadataRequest{ProviderTypeName: "foundrydb"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)

	if resp.TypeName != "foundrydb_database_user" {
		t.Errorf("TypeName = %q; want %q", resp.TypeName, "foundrydb_database_user")
	}
}

// TestUnitDatabaseUserDataSource_Schema_requiredAttributes verifies that service_id and
// username are required inputs.
func TestUnitDatabaseUserDataSource_Schema_requiredAttributes(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	for _, key := range []string{"service_id", "username"} {
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

// TestUnitDatabaseUserDataSource_Schema_computedAttributes verifies computed output fields.
func TestUnitDatabaseUserDataSource_Schema_computedAttributes(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	for _, key := range []string{"password", "host", "port", "database", "connection_string"} {
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

// TestUnitDatabaseUserDataSource_Schema_sensitiveAttributes verifies that password and
// connection_string are marked sensitive.
func TestUnitDatabaseUserDataSource_Schema_sensitiveAttributes(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	for _, key := range []string{"password", "connection_string"} {
		attr, ok := resp.Schema.Attributes[key]
		if !ok {
			t.Errorf("schema missing attribute %q", key)
			continue
		}
		strAttr, ok := attr.(datasourceschema.StringAttribute)
		if !ok {
			t.Errorf("attribute %q is not a StringAttribute; got %T", key, attr)
			continue
		}
		if !strAttr.Sensitive {
			t.Errorf("attribute %q should be marked Sensitive", key)
		}
	}
}

// TestUnitDatabaseUserDataSource_Schema_portIsInt64 verifies the port attribute uses Int64.
func TestUnitDatabaseUserDataSource_Schema_portIsInt64(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	attr, ok := resp.Schema.Attributes["port"]
	if !ok {
		t.Fatal("schema missing 'port' attribute")
	}
	if _, ok := attr.(datasourceschema.Int64Attribute); !ok {
		t.Errorf("'port' should be an Int64Attribute; got %T", attr)
	}
}

// TestUnitDatabaseUserDataSource_Schema_allExpectedFields verifies the complete field set.
func TestUnitDatabaseUserDataSource_Schema_allExpectedFields(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	expected := []string{"service_id", "username", "password", "host", "port", "database", "connection_string"}
	for _, field := range expected {
		if _, ok := resp.Schema.Attributes[field]; !ok {
			t.Errorf("expected attribute %q not found in database_user data source schema", field)
		}
	}
}

// TestUnitDatabaseUserDataSource_Schema_markdownDescription verifies a description is set.
func TestUnitDatabaseUserDataSource_Schema_markdownDescription(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	if resp.Schema.MarkdownDescription == "" {
		t.Error("database_user data source schema MarkdownDescription should not be empty")
	}
}

// TestUnitDatabaseUserDataSource_ImplementsInterface verifies the data source satisfies
// the datasource.DataSource interface.
func TestUnitDatabaseUserDataSource_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ datasource.DataSource = provider.NewDatabaseUserDataSource()
}

// TestUnitDatabaseUserDataSource_NotNil verifies the constructor returns a non-nil value.
func TestUnitDatabaseUserDataSource_NotNil(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()
	if ds == nil {
		t.Fatal("NewDatabaseUserDataSource() returned nil")
	}
}

// TestUnitDatabaseUserDataSource_Configure_nilProviderData verifies Configure handles
// nil provider data gracefully (expected during Terraform's initial configuration phase).
func TestUnitDatabaseUserDataSource_Configure_nilProviderData(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	configurable, ok := ds.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("NewDatabaseUserDataSource() does not implement datasource.DataSourceWithConfigure")
	}

	req := datasource.ConfigureRequest{ProviderData: nil}
	resp := &datasource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with nil provider data should not produce errors; got: %v", resp.Diagnostics)
	}
}

// TestUnitDatabaseUserDataSource_Configure_wrongType verifies Configure returns an error
// when the provider data is not a *foundrydb.Client.
func TestUnitDatabaseUserDataSource_Configure_wrongType(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	configurable, ok := ds.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("NewDatabaseUserDataSource() does not implement datasource.DataSourceWithConfigure")
	}

	req := datasource.ConfigureRequest{ProviderData: struct{ foo string }{"bar"}}
	resp := &datasource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure with wrong provider data type should produce an error")
	}
}

// TestUnitDatabaseUserDataSource_Schema_serviceIDNotComputed verifies that service_id
// is not computed (it is a user input, not populated by the API).
func TestUnitDatabaseUserDataSource_Schema_serviceIDNotComputed(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	attr, ok := resp.Schema.Attributes["service_id"]
	if !ok {
		t.Fatal("service_id attribute not found")
	}
	if attr.IsComputed() {
		t.Error("service_id should not be Computed (it is a user-supplied lookup key)")
	}
}

// TestUnitDatabaseUserDataSource_Schema_usernameNotComputed verifies that username
// is not computed.
func TestUnitDatabaseUserDataSource_Schema_usernameNotComputed(t *testing.T) {
	t.Parallel()
	ds := provider.NewDatabaseUserDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	attr, ok := resp.Schema.Attributes["username"]
	if !ok {
		t.Fatal("username attribute not found")
	}
	if attr.IsComputed() {
		t.Error("username should not be Computed (it is a user-supplied lookup key)")
	}
}
