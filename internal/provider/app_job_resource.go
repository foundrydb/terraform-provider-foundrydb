package provider

import (
	"context"
	"fmt"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure appJobResource satisfies resource.Resource.
var _ resource.Resource = &appJobResource{}

// appJobResource implements the foundrydb_app_job resource.
type appJobResource struct {
	client *foundrydb.Client
}

// appJobResourceModel holds the Terraform state for a foundrydb_app_job.
type appJobResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	AppServiceID        types.String `tfsdk:"app_service_id"`
	Name                types.String `tfsdk:"name"`
	ScheduleCron        types.String `tfsdk:"schedule_cron"`
	Timezone            types.String `tfsdk:"timezone"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	ImageRef            types.String `tfsdk:"image_ref"`
	Command             types.List   `tfsdk:"command"`
	Env                 types.Map    `tfsdk:"env"`
	MaxRetries          types.Int64  `tfsdk:"max_retries"`
	RetryBackoffSeconds types.Int64  `tfsdk:"retry_backoff_seconds"`
	MaxRuntimeSeconds   types.Int64  `tfsdk:"max_runtime_seconds"`
	ConcurrencyCap      types.Int64  `tfsdk:"concurrency_cap"`
}

// NewAppJobResource returns a new appJobResource factory.
func NewAppJobResource() resource.Resource {
	return &appJobResource{}
}

func (r *appJobResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_job"
}

func (r *appJobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a job definition on a FoundryDB app service. A job is a container run (image, command, and environment layered over the app's own configuration) with an optional cron schedule. Jobs without a schedule only run when triggered explicitly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the job.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the app service that owns this job. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique name for the job within the app service. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schedule_cron": schema.StringAttribute{
				MarkdownDescription: "Five-field cron expression (minute granularity; descriptors such as `@daily` are accepted) evaluated in `timezone`. When absent or set to an empty string, the job is unscheduled and only runs on explicit invocation.",
				Optional:            true,
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "IANA timezone name used to evaluate `schedule_cron` (e.g. `UTC`, `Europe/Stockholm`). Defaults to `UTC`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the scheduled job is active. Disabled jobs still run on explicit invocation. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"image_ref": schema.StringAttribute{
				MarkdownDescription: "Container image reference that overrides the app's image for this job (e.g. `registry.example.com/tools:latest`). When absent, the job inherits the app's current image.",
				Optional:            true,
			},
			"command": schema.ListAttribute{
				MarkdownDescription: "Container argv override in exec form (never shell-expanded). Overrides the image's default entrypoint/cmd. Omit to use the image default.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"env": schema.MapAttribute{
				MarkdownDescription: "Environment variables layered on top of the app's environment at dispatch time. Job keys override app keys. Keys are forwarded verbatim.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "Number of times a failed invocation is retried before being marked as permanently failed. Defaults to `0` (no retries).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"retry_backoff_seconds": schema.Int64Attribute{
				MarkdownDescription: "Minimum delay in seconds between retry attempts. Defaults to `0`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_runtime_seconds": schema.Int64Attribute{
				MarkdownDescription: "Maximum wall-clock time in seconds before the platform terminates the invocation. Defaults to `3600` (one hour).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"concurrency_cap": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of simultaneous invocations. A new invocation that would exceed this cap is rejected (409). Defaults to `1`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *appJobResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *appJobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appJobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := foundrydb.AppJobCreateRequest{
		Name: plan.Name.ValueString(),
	}

	if !plan.ScheduleCron.IsNull() && !plan.ScheduleCron.IsUnknown() && plan.ScheduleCron.ValueString() != "" {
		s := plan.ScheduleCron.ValueString()
		createReq.ScheduleCron = &s
	}
	if !plan.Timezone.IsNull() && !plan.Timezone.IsUnknown() {
		createReq.Timezone = plan.Timezone.ValueString()
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		v := plan.Enabled.ValueBool()
		createReq.Enabled = &v
	}
	if !plan.ImageRef.IsNull() && !plan.ImageRef.IsUnknown() && plan.ImageRef.ValueString() != "" {
		s := plan.ImageRef.ValueString()
		createReq.ImageRef = &s
	}
	if !plan.Command.IsNull() && !plan.Command.IsUnknown() {
		var cmds []string
		resp.Diagnostics.Append(plan.Command.ElementsAs(ctx, &cmds, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Command = cmds
	}
	if !plan.Env.IsNull() && !plan.Env.IsUnknown() {
		envMap := make(map[string]string)
		resp.Diagnostics.Append(plan.Env.ElementsAs(ctx, &envMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Env = envMap
	}
	if !plan.MaxRetries.IsNull() && !plan.MaxRetries.IsUnknown() {
		v := int(plan.MaxRetries.ValueInt64())
		createReq.MaxRetries = &v
	}
	if !plan.RetryBackoffSeconds.IsNull() && !plan.RetryBackoffSeconds.IsUnknown() {
		v := int(plan.RetryBackoffSeconds.ValueInt64())
		createReq.RetryBackoffSeconds = &v
	}
	if !plan.MaxRuntimeSeconds.IsNull() && !plan.MaxRuntimeSeconds.IsUnknown() {
		v := int(plan.MaxRuntimeSeconds.ValueInt64())
		createReq.MaxRuntimeSeconds = &v
	}
	if !plan.ConcurrencyCap.IsNull() && !plan.ConcurrencyCap.IsUnknown() {
		v := int(plan.ConcurrencyCap.ValueInt64())
		createReq.ConcurrencyCap = &v
	}

	job, err := r.client.CreateAppJob(ctx, plan.AppServiceID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating app job", err.Error())
		return
	}

	appJobToState(ctx, job, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *appJobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	job, err := r.client.GetAppJob(ctx, state.AppServiceID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading app job", err.Error())
		return
	}
	if job == nil {
		// Resource has been deleted outside of Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	appJobToState(ctx, job, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *appJobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan appJobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state appJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patchReq := foundrydb.AppJobPatchRequest{}

	// schedule_cron: distinguish "set to a value", "clear it", and "leave alone".
	planCron := plan.ScheduleCron
	stateCron := state.ScheduleCron
	if !planCron.Equal(stateCron) {
		if planCron.IsNull() || planCron.IsUnknown() || planCron.ValueString() == "" {
			// Config removed the schedule; send clear_schedule.
			patchReq.ClearSchedule = true
		} else {
			s := planCron.ValueString()
			patchReq.ScheduleCron = &s
		}
	}

	if !plan.Timezone.Equal(state.Timezone) && !plan.Timezone.IsNull() && !plan.Timezone.IsUnknown() {
		s := plan.Timezone.ValueString()
		patchReq.Timezone = &s
	}
	if !plan.Enabled.Equal(state.Enabled) && !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		v := plan.Enabled.ValueBool()
		patchReq.Enabled = &v
	}

	// image_ref: distinguish "set to a value", "clear it", and "leave alone".
	planImage := plan.ImageRef
	stateImage := state.ImageRef
	if !planImage.Equal(stateImage) {
		if planImage.IsNull() || planImage.IsUnknown() || planImage.ValueString() == "" {
			patchReq.ClearImageRef = true
		} else {
			s := planImage.ValueString()
			patchReq.ImageRef = &s
		}
	}

	if !plan.Command.Equal(state.Command) && !plan.Command.IsNull() && !plan.Command.IsUnknown() {
		var cmds []string
		resp.Diagnostics.Append(plan.Command.ElementsAs(ctx, &cmds, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		patchReq.Command = cmds
	}
	if !plan.Env.Equal(state.Env) && !plan.Env.IsNull() && !plan.Env.IsUnknown() {
		envMap := make(map[string]string)
		resp.Diagnostics.Append(plan.Env.ElementsAs(ctx, &envMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		patchReq.Env = envMap
	}
	if !plan.MaxRetries.Equal(state.MaxRetries) && !plan.MaxRetries.IsNull() && !plan.MaxRetries.IsUnknown() {
		v := int(plan.MaxRetries.ValueInt64())
		patchReq.MaxRetries = &v
	}
	if !plan.RetryBackoffSeconds.Equal(state.RetryBackoffSeconds) && !plan.RetryBackoffSeconds.IsNull() && !plan.RetryBackoffSeconds.IsUnknown() {
		v := int(plan.RetryBackoffSeconds.ValueInt64())
		patchReq.RetryBackoffSeconds = &v
	}
	if !plan.MaxRuntimeSeconds.Equal(state.MaxRuntimeSeconds) && !plan.MaxRuntimeSeconds.IsNull() && !plan.MaxRuntimeSeconds.IsUnknown() {
		v := int(plan.MaxRuntimeSeconds.ValueInt64())
		patchReq.MaxRuntimeSeconds = &v
	}
	if !plan.ConcurrencyCap.Equal(state.ConcurrencyCap) && !plan.ConcurrencyCap.IsNull() && !plan.ConcurrencyCap.IsUnknown() {
		v := int(plan.ConcurrencyCap.ValueInt64())
		patchReq.ConcurrencyCap = &v
	}

	job, err := r.client.UpdateAppJob(ctx, state.AppServiceID.ValueString(), state.ID.ValueString(), patchReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating app job", err.Error())
		return
	}

	appJobToState(ctx, job, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *appJobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteAppJob(ctx, state.AppServiceID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting app job", err.Error())
	}
}

// appJobToState maps an API AppJob into a Terraform state model.
func appJobToState(ctx context.Context, job *foundrydb.AppJob, model *appJobResourceModel, diags interface{ HasError() bool }) {
	model.ID = types.StringValue(job.ID)
	model.AppServiceID = types.StringValue(job.ServiceID)
	model.Name = types.StringValue(job.Name)
	model.Timezone = types.StringValue(job.Timezone)
	model.Enabled = types.BoolValue(job.Enabled)
	model.MaxRetries = types.Int64Value(int64(job.MaxRetries))
	model.RetryBackoffSeconds = types.Int64Value(int64(job.RetryBackoffSeconds))
	model.MaxRuntimeSeconds = types.Int64Value(int64(job.MaxRuntimeSeconds))
	model.ConcurrencyCap = types.Int64Value(int64(job.ConcurrencyCap))

	if job.ScheduleCron != nil {
		model.ScheduleCron = types.StringValue(*job.ScheduleCron)
	} else {
		model.ScheduleCron = types.StringNull()
	}

	if job.ImageRef != nil {
		model.ImageRef = types.StringValue(*job.ImageRef)
	} else {
		model.ImageRef = types.StringNull()
	}

	if len(job.Command) > 0 {
		elems := make([]types.String, len(job.Command))
		for i, s := range job.Command {
			elems[i] = types.StringValue(s)
		}
		listVal, _ := types.ListValueFrom(ctx, types.StringType, elems)
		model.Command = listVal
	} else {
		model.Command, _ = types.ListValue(types.StringType, nil)
	}

	if len(job.Env) > 0 {
		envElems := make(map[string]types.String, len(job.Env))
		for k, v := range job.Env {
			envElems[k] = types.StringValue(v)
		}
		mapVal, _ := types.MapValueFrom(ctx, types.StringType, envElems)
		model.Env = mapVal
	} else {
		model.Env, _ = types.MapValue(types.StringType, nil)
	}
}
