package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Ensure appServiceResource satisfies the resource.Resource and
// resource.ResourceWithImportState interfaces.
var _ resource.Resource = &appServiceResource{}
var _ resource.ResourceWithImportState = &appServiceResource{}

// appServiceResource implements the foundrydb_app_service resource.
type appServiceResource struct {
	client *foundrydb.Client
}

// appServiceResourceModel holds the Terraform state for a foundrydb_app_service.
type appServiceResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	PlanName           types.String `tfsdk:"plan_name"`
	Zone               types.String `tfsdk:"zone"`
	StorageSizeGB      types.Int64  `tfsdk:"storage_size_gb"`
	StorageTier        types.String `tfsdk:"storage_tier"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	AppConfig          types.Object `tfsdk:"app_config"`
	AttachedServiceIDs types.List   `tfsdk:"attached_service_ids"`
	Status             types.String `tfsdk:"status"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

// appContainerConfigModel maps the app_config { } block.
type appContainerConfigModel struct {
	ImageRef                    types.String `tfsdk:"image_ref"`
	ContainerPort               types.Int64  `tfsdk:"container_port"`
	Env                         types.Map    `tfsdk:"env"`
	CustomDomains               types.List   `tfsdk:"custom_domains"`
	RegistryUsername            types.String `tfsdk:"registry_username"`
	RegistryPassword            types.String `tfsdk:"registry_password"`
	HealthCheckPath             types.String `tfsdk:"health_check_path"`
	HealthCheckIntervalSeconds  types.Int64  `tfsdk:"health_check_interval_seconds"`
	HealthCheckTimeoutSeconds   types.Int64  `tfsdk:"health_check_timeout_seconds"`
	HealthCheckHealthyThreshold types.Int64  `tfsdk:"health_check_healthy_threshold"`
}

var appContainerConfigAttrTypes = map[string]attr.Type{
	"image_ref":                      types.StringType,
	"container_port":                 types.Int64Type,
	"env":                            types.MapType{ElemType: types.StringType},
	"custom_domains":                 types.ListType{ElemType: types.StringType},
	"registry_username":              types.StringType,
	"registry_password":              types.StringType,
	"health_check_path":              types.StringType,
	"health_check_interval_seconds":  types.Int64Type,
	"health_check_timeout_seconds":   types.Int64Type,
	"health_check_healthy_threshold": types.Int64Type,
}

// NewAppServiceResource returns a new appServiceResource factory.
func NewAppServiceResource() resource.Resource {
	return &appServiceResource{}
}

func (r *appServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_service"
}

func (r *appServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a FoundryDB app service. An app service runs a container workload on managed compute with optional database attachments, custom domains, and health checks. After creation, the provider waits up to 15 minutes for the service to reach `running` status.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the app service.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name for the app service. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plan_name": schema.StringAttribute{
				MarkdownDescription: "Compute tier: `tier-1` through `tier-15`. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "UpCloud zone (e.g. `se-sto1`). Defaults to `se-sto1`. Changing this value destroys and recreates the resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"storage_size_gb": schema.Int64Attribute{
				MarkdownDescription: "Data disk size in gigabytes. Changing this value destroys and recreates the resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"storage_tier": schema.StringAttribute{
				MarkdownDescription: "Storage performance tier: `maxiops` (NVMe SSD, production) or `standard` (HDD, development). Changing this value destroys and recreates the resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Optional organization ID to scope this service to. Changing this value destroys and recreates the resource.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"app_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Container configuration for the app service. All sub-fields can be updated in-place.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"image_ref": schema.StringAttribute{
						MarkdownDescription: "Container image reference (e.g. `registry.example.com/myapp:latest`).",
						Required:            true,
					},
					"container_port": schema.Int64Attribute{
						MarkdownDescription: "TCP port the container listens on for HTTP traffic.",
						Required:            true,
					},
					"env": schema.MapAttribute{
						MarkdownDescription: "Environment variables injected into the container at runtime.",
						Optional:            true,
						Computed:            true,
						ElementType:         types.StringType,
						PlanModifiers: []planmodifier.Map{
							mapplanmodifier.UseStateForUnknown(),
						},
					},
					"custom_domains": schema.ListAttribute{
						MarkdownDescription: "Custom domain names to route to this app service (e.g. `app.example.com`).",
						Optional:            true,
						Computed:            true,
						ElementType:         types.StringType,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.UseStateForUnknown(),
						},
					},
					"registry_username": schema.StringAttribute{
						MarkdownDescription: "Username for authenticating against a private container registry.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"registry_password": schema.StringAttribute{
						MarkdownDescription: "Password for authenticating against a private container registry. Write-only: stored in the platform secret store and never returned by the API.",
						Optional:            true,
						Computed:            true,
						Sensitive:           true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"health_check_path": schema.StringAttribute{
						MarkdownDescription: "HTTP path the platform polls to determine container health (e.g. `/healthz`). Defaults to `/`.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"health_check_interval_seconds": schema.Int64Attribute{
						MarkdownDescription: "Seconds between health check polls. Defaults to `30`.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.UseStateForUnknown(),
						},
					},
					"health_check_timeout_seconds": schema.Int64Attribute{
						MarkdownDescription: "Seconds before an individual health check attempt times out. Defaults to `5`.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.UseStateForUnknown(),
						},
					},
					"health_check_healthy_threshold": schema.Int64Attribute{
						MarkdownDescription: "Consecutive successful health checks required to mark the container healthy. Defaults to `2`.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.UseStateForUnknown(),
						},
					},
				},
			},
			"attached_service_ids": schema.ListAttribute{
				MarkdownDescription: "UUIDs of managed database or other services to attach to this app service.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current lifecycle status of the app service (e.g. `running`, `provisioning`).",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of when the app service was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of the last app service update.",
				Computed:            true,
			},
		},
	}
}

func (r *appServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	switch v := req.ProviderData.(type) {
	case *providerData:
		r.client = v.client
	case *foundrydb.Client:
		// Accepted for backward compatibility with tests that inject a bare client.
		r.client = v
	default:
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *providerData, got %T", req.ProviderData),
		)
	}
}

func (r *appServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appConfig, diags := planToAppContainerConfig(ctx, plan.AppConfig)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := foundrydb.CreateAppServiceRequest{
		Name:      plan.Name.ValueString(),
		PlanName:  plan.PlanName.ValueString(),
		AppConfig: appConfig,
	}
	if !plan.Zone.IsNull() && !plan.Zone.IsUnknown() {
		createReq.Zone = plan.Zone.ValueString()
	}
	if !plan.StorageSizeGB.IsNull() && !plan.StorageSizeGB.IsUnknown() {
		createReq.StorageSizeGB = int(plan.StorageSizeGB.ValueInt64())
	}
	if !plan.StorageTier.IsNull() && !plan.StorageTier.IsUnknown() {
		createReq.StorageTier = plan.StorageTier.ValueString()
	}
	if !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() {
		createReq.OrganizationID = plan.OrganizationID.ValueString()
	}
	if !plan.AttachedServiceIDs.IsNull() && !plan.AttachedServiceIDs.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(plan.AttachedServiceIDs.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.AttachedServiceIDs = ids
	}

	svc, err := r.client.CreateAppService(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating app service", err.Error())
		return
	}

	// Wait for the service to reach running status before returning.
	svc, err = r.client.WaitForAppRunning(ctx, svc.ID, 15*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for app service to become running", err.Error())
		return
	}

	appServiceToState(ctx, svc, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *appServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.client.GetAppService(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading app service", err.Error())
		return
	}
	if svc == nil {
		// Resource has been deleted outside of Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	appServiceToState(ctx, svc, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *appServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan appServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state appServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appConfig, diags := planToAppContainerConfig(ctx, plan.AppConfig)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := foundrydb.UpdateAppServiceRequest{
		AppConfig: appConfig,
	}

	svc, err := r.client.UpdateAppService(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating app service", err.Error())
		return
	}

	appServiceToState(ctx, svc, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *appServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteAppService(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting app service", err.Error())
	}
}

// ImportState implements resource.ResourceWithImportState. The import ID is the app service UUID.
func (r *appServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Set just the ID; Read will populate the rest.
	resp.Diagnostics.Append(resp.State.Set(ctx, &appServiceResourceModel{
		ID: types.StringValue(req.ID),
	})...)
	if resp.Diagnostics.HasError() {
		return
	}

	readReq := resource.ReadRequest{State: tfsdk.State(resp.State)}
	readResp := &resource.ReadResponse{State: resp.State}
	r.Read(ctx, readReq, readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
	resp.State = readResp.State
}

// planToAppContainerConfig decodes the app_config Terraform object into an SDK AppContainerConfig.
// The registry_password is write-only; it is taken from the plan value so it can be sent on
// create/update even though the API never returns it.
func planToAppContainerConfig(ctx context.Context, obj types.Object) (foundrydb.AppContainerConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var cfg appContainerConfigModel
	diags.Append(obj.As(ctx, &cfg, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return foundrydb.AppContainerConfig{}, diags
	}

	out := foundrydb.AppContainerConfig{
		ImageRef:      cfg.ImageRef.ValueString(),
		ContainerPort: int(cfg.ContainerPort.ValueInt64()),
	}

	if !cfg.Env.IsNull() && !cfg.Env.IsUnknown() {
		envMap := make(map[string]string)
		diags.Append(cfg.Env.ElementsAs(ctx, &envMap, false)...)
		if diags.HasError() {
			return out, diags
		}
		out.Env = envMap
	}

	if !cfg.CustomDomains.IsNull() && !cfg.CustomDomains.IsUnknown() {
		var domains []string
		diags.Append(cfg.CustomDomains.ElementsAs(ctx, &domains, false)...)
		if diags.HasError() {
			return out, diags
		}
		out.CustomDomains = domains
	}

	if !cfg.RegistryUsername.IsNull() && !cfg.RegistryUsername.IsUnknown() {
		out.RegistryUsername = cfg.RegistryUsername.ValueString()
	}

	if !cfg.RegistryPassword.IsNull() && !cfg.RegistryPassword.IsUnknown() {
		out.RegistryPassword = cfg.RegistryPassword.ValueString()
	}

	if !cfg.HealthCheckPath.IsNull() && !cfg.HealthCheckPath.IsUnknown() {
		out.HealthCheckPath = cfg.HealthCheckPath.ValueString()
	}

	if !cfg.HealthCheckIntervalSeconds.IsNull() && !cfg.HealthCheckIntervalSeconds.IsUnknown() {
		out.HealthCheckIntervalSeconds = int(cfg.HealthCheckIntervalSeconds.ValueInt64())
	}

	if !cfg.HealthCheckTimeoutSeconds.IsNull() && !cfg.HealthCheckTimeoutSeconds.IsUnknown() {
		out.HealthCheckTimeoutSeconds = int(cfg.HealthCheckTimeoutSeconds.ValueInt64())
	}

	if !cfg.HealthCheckHealthyThreshold.IsNull() && !cfg.HealthCheckHealthyThreshold.IsUnknown() {
		out.HealthCheckHealthyThreshold = int(cfg.HealthCheckHealthyThreshold.ValueInt64())
	}

	return out, diags
}

// appServiceToState maps an API AppService into a Terraform state model.
// registry_password is never returned by the API; the existing value from the model is preserved
// so Terraform does not show perpetual drift on that sensitive field.
func appServiceToState(ctx context.Context, svc *foundrydb.AppService, model *appServiceResourceModel, diags *diag.Diagnostics) {
	model.ID = types.StringValue(svc.ID)
	model.Name = types.StringValue(svc.Name)
	model.PlanName = types.StringValue(svc.PlanName)
	model.Zone = types.StringValue(svc.Zone)
	model.StorageSizeGB = types.Int64Value(int64(svc.StorageSizeGB))
	model.StorageTier = types.StringValue(svc.StorageTier)
	model.Status = types.StringValue(svc.Status)
	model.CreatedAt = types.StringValue(svc.CreatedAt)
	model.UpdatedAt = types.StringValue(svc.UpdatedAt)

	// organization_id is not returned by the API; preserve whatever was set in state/plan.

	if len(svc.AttachedServiceIDs) > 0 {
		elems := make([]attr.Value, len(svc.AttachedServiceIDs))
		for i, id := range svc.AttachedServiceIDs {
			elems[i] = types.StringValue(id)
		}
		listVal, d := types.ListValue(types.StringType, elems)
		diags.Append(d...)
		model.AttachedServiceIDs = listVal
	} else {
		emptyList, d := types.ListValue(types.StringType, []attr.Value{})
		diags.Append(d...)
		model.AttachedServiceIDs = emptyList
	}

	if svc.AppConfig == nil {
		// No app config returned; preserve the model as-is to avoid clearing known state.
		return
	}

	// Preserve the registry_password from the existing model state. The API never returns it.
	preservedPassword := types.StringValue("")
	if !model.AppConfig.IsNull() && !model.AppConfig.IsUnknown() {
		var existing appContainerConfigModel
		if dd := model.AppConfig.As(ctx, &existing, basetypes.ObjectAsOptions{}); !dd.HasError() {
			preservedPassword = existing.RegistryPassword
		}
	}

	apiCfg := svc.AppConfig

	envVal, d := envMapToTF(ctx, apiCfg.Env)
	diags.Append(d...)

	customDomainsVal, d := stringSliceToList(apiCfg.CustomDomains)
	diags.Append(d...)

	appConfigObj, d := types.ObjectValue(appContainerConfigAttrTypes, map[string]attr.Value{
		"image_ref":                      types.StringValue(apiCfg.ImageRef),
		"container_port":                 types.Int64Value(int64(apiCfg.ContainerPort)),
		"env":                            envVal,
		"custom_domains":                 customDomainsVal,
		"registry_username":              types.StringValue(apiCfg.RegistryUsername),
		"registry_password":              preservedPassword,
		"health_check_path":              types.StringValue(apiCfg.HealthCheckPath),
		"health_check_interval_seconds":  types.Int64Value(int64(apiCfg.HealthCheckIntervalSeconds)),
		"health_check_timeout_seconds":   types.Int64Value(int64(apiCfg.HealthCheckTimeoutSeconds)),
		"health_check_healthy_threshold": types.Int64Value(int64(apiCfg.HealthCheckHealthyThreshold)),
	})
	diags.Append(d...)
	model.AppConfig = appConfigObj
}

// envMapToTF converts a Go map[string]string into a Terraform types.Map.
func envMapToTF(ctx context.Context, m map[string]string) (types.Map, diag.Diagnostics) {
	if len(m) == 0 {
		return types.MapValueMust(types.StringType, map[string]attr.Value{}), nil
	}
	elems := make(map[string]types.String, len(m))
	for k, v := range m {
		elems[k] = types.StringValue(v)
	}
	return types.MapValueFrom(ctx, types.StringType, elems)
}

// stringSliceToList converts a []string into a Terraform types.List of strings.
func stringSliceToList(ss []string) (types.List, diag.Diagnostics) {
	elems := make([]attr.Value, len(ss))
	for i, s := range ss {
		elems[i] = types.StringValue(s)
	}
	return types.ListValue(types.StringType, elems)
}
