package provider_test

import (
	"context"
	"testing"

	"github.com/anorph/terraform-provider-foundrydb/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// TestUnitOrganizationsDataSource_Metadata verifies the data source type name.
func TestUnitOrganizationsDataSource_Metadata(t *testing.T) {
	t.Parallel()
	ds := provider.NewOrganizationsDataSource()

	req := datasource.MetadataRequest{ProviderTypeName: "foundrydb"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)

	if resp.TypeName != "foundrydb_organizations" {
		t.Errorf("TypeName = %q; want %q", resp.TypeName, "foundrydb_organizations")
	}
}

// TestUnitOrganizationsDataSource_Schema verifies the data source schema.
func TestUnitOrganizationsDataSource_Schema(t *testing.T) {
	t.Parallel()
	ds := provider.NewOrganizationsDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	// The top-level attribute is the organizations list.
	orgsAttr, ok := resp.Schema.Attributes["organizations"]
	if !ok {
		t.Fatal("schema missing 'organizations' attribute")
	}

	// It must be computed (populated by the API).
	if !orgsAttr.IsComputed() {
		t.Error("'organizations' should be Computed")
	}
	if orgsAttr.IsRequired() {
		t.Error("'organizations' should not be Required")
	}
}

// TestUnitOrganizationsDataSource_Schema_nestedAttributes verifies nested org fields.
func TestUnitOrganizationsDataSource_Schema_nestedAttributes(t *testing.T) {
	t.Parallel()
	ds := provider.NewOrganizationsDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	// Retrieve the nested attribute object from the ListNestedAttribute.
	orgsAttr, ok := resp.Schema.Attributes["organizations"]
	if !ok {
		t.Fatal("organizations attribute not found")
	}

	listNested, ok := orgsAttr.(datasourceschema.ListNestedAttribute)
	if !ok {
		t.Fatalf("organizations is not a ListNestedAttribute; got %T", orgsAttr)
	}

	nestedAttrs := listNested.NestedObject.Attributes

	expectedFields := []string{"id", "name", "slug", "role", "created_at"}
	for _, field := range expectedFields {
		if _, ok := nestedAttrs[field]; !ok {
			t.Errorf("nested organization object missing field %q", field)
		}
	}
}

// TestUnitOrganizationsDataSource_Schema_nestedAttributesComputed verifies that all
// nested organization attributes are computed (read-only from the API).
func TestUnitOrganizationsDataSource_Schema_nestedAttributesComputed(t *testing.T) {
	t.Parallel()
	ds := provider.NewOrganizationsDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	orgsAttr, ok := resp.Schema.Attributes["organizations"]
	if !ok {
		t.Fatal("organizations attribute not found")
	}
	listNested, ok := orgsAttr.(datasourceschema.ListNestedAttribute)
	if !ok {
		t.Fatalf("organizations is not a ListNestedAttribute; got %T", orgsAttr)
	}

	for field, attr := range listNested.NestedObject.Attributes {
		if !attr.IsComputed() {
			t.Errorf("nested field %q should be Computed", field)
		}
	}
}

// TestUnitOrganizationsDataSource_Schema_markdownDescription verifies a description is set.
func TestUnitOrganizationsDataSource_Schema_markdownDescription(t *testing.T) {
	t.Parallel()
	ds := provider.NewOrganizationsDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	if resp.Schema.MarkdownDescription == "" {
		t.Error("organizations data source schema MarkdownDescription should not be empty")
	}
}

// TestUnitOrganizationsDataSource_ImplementsInterface verifies the data source satisfies
// the datasource.DataSource interface.
func TestUnitOrganizationsDataSource_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ datasource.DataSource = provider.NewOrganizationsDataSource()
}

// TestUnitOrganizationsDataSource_NotNil verifies the constructor returns a non-nil value.
func TestUnitOrganizationsDataSource_NotNil(t *testing.T) {
	t.Parallel()
	ds := provider.NewOrganizationsDataSource()
	if ds == nil {
		t.Fatal("NewOrganizationsDataSource() returned nil")
	}
}

// TestUnitOrganizationsDataSource_Configure_nilProviderData verifies Configure does not
// produce errors when provider data is nil.
func TestUnitOrganizationsDataSource_Configure_nilProviderData(t *testing.T) {
	t.Parallel()
	ds := provider.NewOrganizationsDataSource()

	configurable, ok := ds.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("NewOrganizationsDataSource() does not implement datasource.DataSourceWithConfigure")
	}

	req := datasource.ConfigureRequest{ProviderData: nil}
	resp := &datasource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with nil provider data should not produce errors; got: %v", resp.Diagnostics)
	}
}

// TestUnitOrganizationsDataSource_Configure_wrongType verifies Configure returns an error
// when the provider data is not a *foundrydb.Client.
func TestUnitOrganizationsDataSource_Configure_wrongType(t *testing.T) {
	t.Parallel()
	ds := provider.NewOrganizationsDataSource()

	configurable, ok := ds.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("NewOrganizationsDataSource() does not implement datasource.DataSourceWithConfigure")
	}

	req := datasource.ConfigureRequest{ProviderData: 42}
	resp := &datasource.ConfigureResponse{}
	configurable.Configure(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure with wrong provider data type should produce an error")
	}
}
