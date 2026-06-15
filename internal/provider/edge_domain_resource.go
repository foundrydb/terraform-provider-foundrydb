package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure edgeDomainResource satisfies resource.Resource.
var _ resource.Resource = &edgeDomainResource{}

// edgeDomainResource implements the foundrydb_edge_domain resource.
type edgeDomainResource struct {
	edge *edgeClient
}

// edgeDomainResourceModel holds the Terraform state for a foundrydb_edge_domain.
type edgeDomainResourceModel struct {
	ID           types.String `tfsdk:"id"`
	AppServiceID types.String `tfsdk:"app_service_id"`
	Domain       types.String `tfsdk:"domain"`
	Status       types.String `tfsdk:"status"`
	CNAMETarget  types.String `tfsdk:"cname_target"`
}

// NewEdgeDomainResource returns a new edgeDomainResource factory.
func NewEdgeDomainResource() resource.Resource {
	return &edgeDomainResource{}
}

func (r *edgeDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_domain"
}

func (r *edgeDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches a custom domain to a FoundryDB app service through the edge tier. After creation the platform provisions a TLS certificate and propagates the domain configuration to the edge fleet. The `cname_target` attribute returns the platform hostname that the customer must point their DNS CNAME record at. All attributes are create-time only; changing any attribute destroys and recreates the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the edge domain.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the app service to attach this domain to. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Fully qualified domain name (e.g. `app.example.com`) to attach to the app service. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current lifecycle status of the domain: `pending_verification`, `verifying`, `issuing_certificate`, `propagating`, `active`, `failed`, or `deleting`.",
				Computed:            true,
			},
			"cname_target": schema.StringAttribute{
				MarkdownDescription: "Platform hostname that the custom domain's DNS CNAME record must point at. Populated once the domain has been accepted by the edge tier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *edgeDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *providerData, got %T", req.ProviderData),
		)
		return
	}
	r.edge = pd.edgeClient
}

func (r *edgeDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan edgeDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	d, err := r.edge.CreateDomain(ctx, plan.AppServiceID.ValueString(), plan.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating edge domain", err.Error())
		return
	}

	edgeDomainToState(d, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *edgeDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state edgeDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	d, err := r.edge.GetDomain(ctx, state.AppServiceID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading edge domain", err.Error())
		return
	}
	if d == nil {
		// Domain was deleted outside of Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	edgeDomainToState(d, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is not implemented: all attributes are ForceNew. The framework will
// never call Update because every attribute change triggers a replacement.
func (r *edgeDomainResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Edge domain update not supported",
		"All edge domain attributes require replacement. This is a provider implementation error if this method is called.",
	)
}

func (r *edgeDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state edgeDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.edge.DeleteDomain(ctx, state.AppServiceID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting edge domain", err.Error())
	}
}

// edgeDomainToState maps an API EdgeDomain into the Terraform state model.
func edgeDomainToState(d *EdgeDomain, model *edgeDomainResourceModel) {
	model.ID = types.StringValue(d.ID)
	model.AppServiceID = types.StringValue(d.ServiceID)
	model.Domain = types.StringValue(d.Domain)
	model.Status = types.StringValue(d.Status)
	model.CNAMETarget = types.StringValue(d.CNAMETarget)
}
