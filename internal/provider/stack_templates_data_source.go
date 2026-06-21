package provider

import (
	"context"
	"fmt"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure stackTemplatesDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &stackTemplatesDataSource{}

// stackTemplatesDataSource implements the foundrydb_stack_templates data source.
type stackTemplatesDataSource struct {
	client *foundrydb.Client
}

// stackTemplateSummaryModel represents one entry from the stack template catalog.
type stackTemplateSummaryModel struct {
	Name          types.String  `tfsdk:"name"`
	DisplayName   types.String  `tfsdk:"display_name"`
	Description   types.String  `tfsdk:"description"`
	Version       types.String  `tfsdk:"version"`
	MonthlyTotal  types.Float64 `tfsdk:"monthly_total"`
}

// stackTemplatesDataSourceModel holds the Terraform state for foundrydb_stack_templates.
type stackTemplatesDataSourceModel struct {
	Templates []stackTemplateSummaryModel `tfsdk:"templates"`
}

// NewStackTemplatesDataSource returns a new stackTemplatesDataSource factory.
func NewStackTemplatesDataSource() datasource.DataSource {
	return &stackTemplatesDataSource{}
}

func (d *stackTemplatesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack_templates"
}

func (d *stackTemplatesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all available first-party stack templates from the FoundryDB catalog. Each template includes a fresh cost preview reflecting current plan prices. Use the `name` and `monthly_total` values to populate a `foundrydb_stack` resource.",
		Attributes: map[string]schema.Attribute{
			"templates": schema.ListNestedAttribute{
				MarkdownDescription: "List of available stack templates.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Machine-readable template name used as `template_name` when launching a stack (e.g. `rag-chatbot`).",
							Computed:            true,
						},
						"display_name": schema.StringAttribute{
							MarkdownDescription: "Human-readable name for the template (e.g. `Launch a RAG chatbot`).",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "Short description of what this stack provides.",
							Computed:            true,
						},
						"version": schema.StringAttribute{
							MarkdownDescription: "Semantic version of the template descriptor.",
							Computed:            true,
						},
						"monthly_total": schema.Float64Attribute{
							MarkdownDescription: "Estimated total monthly cost in USD for all resources composed by this template. Pass this value as `accepted_monthly_cost` when creating a `foundrydb_stack` resource.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *stackTemplatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	switch v := req.ProviderData.(type) {
	case *providerData:
		d.client = v.client
	case *foundrydb.Client:
		d.client = v
	default:
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *providerData, got %T", req.ProviderData),
		)
	}
}

func (d *stackTemplatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state stackTemplatesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	templates, err := d.client.ListStackTemplates(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing stack templates",
			fmt.Sprintf("Could not retrieve stack templates: %s", err.Error()),
		)
		return
	}

	state.Templates = make([]stackTemplateSummaryModel, len(templates))
	for i, tmpl := range templates {
		var monthlyTotal float64
		if tmpl.CostPreview != nil {
			monthlyTotal = tmpl.CostPreview.MonthlyTotal
		}
		state.Templates[i] = stackTemplateSummaryModel{
			Name:         types.StringValue(tmpl.Name),
			DisplayName:  types.StringValue(tmpl.DisplayName),
			Description:  types.StringValue(tmpl.Description),
			Version:      types.StringValue(tmpl.Version),
			MonthlyTotal: types.Float64Value(monthlyTotal),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// stackTemplateAttrTypes is the attr.Type map for a stack template object.
// Exported as a package-level var for use in tests.
var stackTemplateAttrTypes = map[string]attr.Type{
	"name":          types.StringType,
	"display_name":  types.StringType,
	"description":   types.StringType,
	"version":       types.StringType,
	"monthly_total": types.Float64Type,
}
