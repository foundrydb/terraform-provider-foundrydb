package provider_test

import (
	"context"
	"testing"

	"github.com/anorph/terraform-provider-foundrydb/internal/provider"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
)

// TestUnitProvider_Metadata verifies the provider type name and version are set correctly.
func TestUnitProvider_Metadata(t *testing.T) {
	t.Parallel()
	p := provider.New("1.2.3")()

	req := fwprovider.MetadataRequest{}
	resp := &fwprovider.MetadataResponse{}
	p.Metadata(context.Background(), req, resp)

	if resp.TypeName != "foundrydb" {
		t.Errorf("TypeName = %q; want %q", resp.TypeName, "foundrydb")
	}
	if resp.Version != "1.2.3" {
		t.Errorf("Version = %q; want %q", resp.Version, "1.2.3")
	}
}

// TestUnitProvider_Metadata_devVersion verifies "dev" version is preserved.
func TestUnitProvider_Metadata_devVersion(t *testing.T) {
	t.Parallel()
	p := provider.New("dev")()

	req := fwprovider.MetadataRequest{}
	resp := &fwprovider.MetadataResponse{}
	p.Metadata(context.Background(), req, resp)

	if resp.Version != "dev" {
		t.Errorf("Version = %q; want %q", resp.Version, "dev")
	}
}

// TestUnitProvider_Schema verifies the provider schema contains all required attributes
// with correct optionality.
func TestUnitProvider_Schema(t *testing.T) {
	t.Parallel()
	p := provider.New("dev")()

	req := fwprovider.SchemaRequest{}
	resp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	attrs := resp.Schema.Attributes

	// All three provider attributes must be present.
	for _, key := range []string{"api_url", "username", "password"} {
		if _, ok := attrs[key]; !ok {
			t.Errorf("schema is missing required attribute %q", key)
		}
	}

	// api_url must be optional (users may rely on the default).
	assertAttrOptional(t, attrs, "api_url")
	// username and password are required credentials.
	assertAttrRequired(t, attrs, "username")
	assertAttrRequired(t, attrs, "password")
}

// TestUnitProvider_Schema_passwordSensitive verifies that the password attribute is marked
// sensitive so that Terraform does not log or display it in plain text.
func TestUnitProvider_Schema_passwordSensitive(t *testing.T) {
	t.Parallel()
	p := provider.New("dev")()

	req := fwprovider.SchemaRequest{}
	resp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["password"]
	if !ok {
		t.Fatal("password attribute not found in schema")
	}
	strAttr, ok := attr.(providerschema.StringAttribute)
	if !ok {
		t.Fatalf("password is not a StringAttribute; got %T", attr)
	}
	if !strAttr.Sensitive {
		t.Error("password attribute should be marked Sensitive")
	}
}

// TestUnitProvider_Schema_markdownDescription verifies the provider-level description is set.
func TestUnitProvider_Schema_markdownDescription(t *testing.T) {
	t.Parallel()
	p := provider.New("dev")()

	req := fwprovider.SchemaRequest{}
	resp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), req, resp)

	if resp.Schema.MarkdownDescription == "" {
		t.Error("provider schema MarkdownDescription should not be empty")
	}
}

// TestUnitProvider_ImplementsProviderInterface asserts the returned value satisfies
// the provider.Provider interface required by the Terraform plugin framework.
func TestUnitProvider_ImplementsProviderInterface(t *testing.T) {
	t.Parallel()
	var _ fwprovider.Provider = provider.New("dev")()
}

// assertAttrRequired is a test helper that fails if an attribute is not marked Required.
func assertAttrRequired(t *testing.T, attrs map[string]providerschema.Attribute, key string) {
	t.Helper()
	attr, ok := attrs[key]
	if !ok {
		t.Errorf("attribute %q not found in schema", key)
		return
	}
	if !attr.IsRequired() {
		t.Errorf("attribute %q should be Required", key)
	}
}

// assertAttrOptional is a test helper that fails if an attribute is marked Required.
func assertAttrOptional(t *testing.T, attrs map[string]providerschema.Attribute, key string) {
	t.Helper()
	attr, ok := attrs[key]
	if !ok {
		t.Errorf("attribute %q not found in schema", key)
		return
	}
	if attr.IsRequired() {
		t.Errorf("attribute %q should be Optional, not Required", key)
	}
}
