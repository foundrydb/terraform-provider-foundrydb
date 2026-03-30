package provider

import (
	"context"
	"fmt"

	"github.com/anorph/terraform-provider-foundrydb/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure organizationsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &organizationsDataSource{}

// organizationsDataSource implements the foundrydb_organizations data source.
type organizationsDataSource struct {
	client *client.Client
}

// organizationModel represents a single organization in Terraform state.
type organizationModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Slug      types.String `tfsdk:"slug"`
	Role      types.String `tfsdk:"role"`
	CreatedAt types.String `tfsdk:"created_at"`
}

// organizationsDataSourceModel holds the Terraform state for foundrydb_organizations.
type organizationsDataSourceModel struct {
	Organizations []organizationModel `tfsdk:"organizations"`
}

// NewOrganizationsDataSource returns a new organizationsDataSource factory.
func NewOrganizationsDataSource() datasource.DataSource {
	return &organizationsDataSource{}
}

func (d *organizationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organizations"
}

func (d *organizationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	orgAttrs := map[string]attr.Type{
		"id":         types.StringType,
		"name":       types.StringType,
		"slug":       types.StringType,
		"role":       types.StringType,
		"created_at": types.StringType,
	}
	_ = orgAttrs // used only for object type reference

	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all organizations the authenticated user belongs to. Use the `id` of the desired organization as `organization_id` on `foundrydb_service` resources to scope them to that organization.",
		Attributes: map[string]schema.Attribute{
			"organizations": schema.ListNestedAttribute{
				MarkdownDescription: "List of organizations.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Unique identifier of the organization.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Display name of the organization.",
							Computed:            true,
						},
						"slug": schema.StringAttribute{
							MarkdownDescription: "URL-friendly slug for the organization.",
							Computed:            true,
						},
						"role": schema.StringAttribute{
							MarkdownDescription: "The authenticated user's role in this organization (e.g. `owner`, `member`).",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "RFC3339 timestamp of when the organization was created.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *organizationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *organizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state organizationsDataSourceModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgs, err := d.client.ListOrganizations()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing organizations",
			fmt.Sprintf("Could not retrieve organizations: %s", err.Error()),
		)
		return
	}

	state.Organizations = make([]organizationModel, len(orgs))
	for i, org := range orgs {
		state.Organizations[i] = organizationModel{
			ID:        types.StringValue(org.ID),
			Name:      types.StringValue(org.Name),
			Slug:      types.StringValue(org.Slug),
			Role:      types.StringValue(org.Role),
			CreatedAt: types.StringValue(org.CreatedAt),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
