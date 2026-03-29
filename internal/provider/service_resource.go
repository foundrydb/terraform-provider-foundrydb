package provider

import (
	"context"
	"fmt"

	"github.com/anorph/terraform-provider-foundrydb/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure serviceResource satisfies the resource.Resource and
// resource.ResourceWithImportState interfaces.
var _ resource.Resource = &serviceResource{}
var _ resource.ResourceWithImportState = &serviceResource{}

// serviceResource implements the foundrydb_service resource.
type serviceResource struct {
	client *client.Client
}

// serviceResourceModel holds the Terraform state for a foundrydb_service.
type serviceResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	DatabaseType  types.String `tfsdk:"database_type"`
	Version       types.String `tfsdk:"version"`
	PlanName      types.String `tfsdk:"plan_name"`
	Zone          types.String `tfsdk:"zone"`
	StorageSizeGB types.Int64  `tfsdk:"storage_size_gb"`
	StorageTier   types.String `tfsdk:"storage_tier"`
	AllowedCIDRs  types.List   `tfsdk:"allowed_cidrs"`
	Status        types.String `tfsdk:"status"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

// NewServiceResource returns a new serviceResource factory.
func NewServiceResource() resource.Resource {
	return &serviceResource{}
}

func (r *serviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *serviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a FoundryDB managed database service. Supports PostgreSQL, MySQL, MongoDB, Valkey, and Kafka. After creation, the provider waits up to 15 minutes for the service to reach `running` status.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the service.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name for the service.",
				Required:            true,
			},
			"database_type": schema.StringAttribute{
				MarkdownDescription: "Database engine. One of: `postgresql`, `mysql`, `mongodb`, `valkey`, `kafka`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Database engine version (e.g. `17` for PostgreSQL 17).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plan_name": schema.StringAttribute{
				MarkdownDescription: "Compute tier: `tier-1` through `tier-15`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "UpCloud zone (e.g. `se-sto1`). Defaults to `se-sto1`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"storage_size_gb": schema.Int64Attribute{
				MarkdownDescription: "Data disk size in gigabytes.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"storage_tier": schema.StringAttribute{
				MarkdownDescription: "Storage performance tier: `maxiops` (NVMe SSD, production) or `standard` (HDD, development).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"allowed_cidrs": schema.ListAttribute{
				MarkdownDescription: "List of CIDR blocks allowed to connect to the database service.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current lifecycle status of the service (e.g. `running`, `provisioning`).",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of when the service was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *serviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *serviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.ServiceCreateRequest{
		Name:         plan.Name.ValueString(),
		DatabaseType: plan.DatabaseType.ValueString(),
		PlanName:     plan.PlanName.ValueString(),
	}
	if !plan.Version.IsNull() && !plan.Version.IsUnknown() {
		createReq.Version = plan.Version.ValueString()
	}
	if !plan.Zone.IsNull() && !plan.Zone.IsUnknown() {
		createReq.Zone = plan.Zone.ValueString()
	}
	if !plan.StorageSizeGB.IsNull() && !plan.StorageSizeGB.IsUnknown() {
		size := plan.StorageSizeGB.ValueInt64()
		createReq.StorageSizeGB = &size
	}
	if !plan.StorageTier.IsNull() && !plan.StorageTier.IsUnknown() {
		createReq.StorageTier = plan.StorageTier.ValueString()
	}
	if !plan.AllowedCIDRs.IsNull() && !plan.AllowedCIDRs.IsUnknown() {
		var cidrs []string
		diags = plan.AllowedCIDRs.ElementsAs(ctx, &cidrs, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.AllowedCIDRs = cidrs
	}

	svc, err := r.client.CreateService(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating service", err.Error())
		return
	}

	// Wait for the service to reach running status before returning.
	svc, err = r.client.WaitForServiceRunning(svc.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for service to become running", err.Error())
		return
	}

	serviceToState(ctx, svc, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.client.GetService(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading service", err.Error())
		return
	}
	if svc == nil {
		// Resource has been deleted outside of Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	serviceToState(ctx, svc, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state serviceResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.ServiceUpdateRequest{}

	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.Name = &name
	}

	if !plan.AllowedCIDRs.Equal(state.AllowedCIDRs) {
		var cidrs []string
		diags = plan.AllowedCIDRs.ElementsAs(ctx, &cidrs, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.AllowedCIDRs = cidrs
	}

	svc, err := r.client.UpdateService(state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating service", err.Error())
		return
	}

	serviceToState(ctx, svc, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteService(state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting service", err.Error())
	}
}

// ImportState implements resource.ResourceWithImportState. The import ID is the service UUID.
func (r *serviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Set just the ID; Read will populate the rest.
	resp.Diagnostics.Append(resp.State.Set(ctx, &serviceResourceModel{
		ID: types.StringValue(req.ID),
	})...)

	// Immediately call Read to fill in all attributes.
	readReq := resource.ReadRequest{State: tfsdk.State(resp.State)}
	readResp := &resource.ReadResponse{State: resp.State}
	r.Read(ctx, readReq, readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
	resp.State = readResp.State
}

// serviceToState maps an API Service into a Terraform state model.
func serviceToState(_ context.Context, svc *client.Service, model *serviceResourceModel, _ *diag.Diagnostics) {
	model.ID = types.StringValue(svc.ID)
	model.Name = types.StringValue(svc.Name)
	model.DatabaseType = types.StringValue(svc.DatabaseType)
	model.Version = types.StringValue(svc.Version)
	model.PlanName = types.StringValue(svc.PlanName)
	model.Zone = types.StringValue(svc.Zone)
	model.StorageSizeGB = types.Int64Value(svc.StorageSizeGB)
	model.StorageTier = types.StringValue(svc.StorageTier)
	model.Status = types.StringValue(svc.Status)
	model.CreatedAt = types.StringValue(svc.CreatedAt)

	cidrElems := make([]attr.Value, len(svc.AllowedCIDRs))
	for i, c := range svc.AllowedCIDRs {
		cidrElems[i] = types.StringValue(c)
	}
	model.AllowedCIDRs, _ = types.ListValue(types.StringType, cidrElems)
}
