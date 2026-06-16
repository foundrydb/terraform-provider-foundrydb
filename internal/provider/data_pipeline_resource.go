package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Ensure dataPipelineResource satisfies the resource.Resource and
// resource.ResourceWithImportState interfaces.
var _ resource.Resource = &dataPipelineResource{}
var _ resource.ResourceWithImportState = &dataPipelineResource{}

// dataPipelineResource implements the foundrydb_data_pipeline resource.
type dataPipelineResource struct {
	client *foundrydb.Client
}

// dataPipelineResourceModel holds the Terraform state for a foundrydb_data_pipeline.
type dataPipelineResourceModel struct {
	ID              types.String `tfsdk:"id"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	Name            types.String `tfsdk:"name"`
	PipelineType    types.String `tfsdk:"pipeline_type"`
	SourceServiceID types.String `tfsdk:"source_service_id"`
	SinkServiceID   types.String `tfsdk:"sink_service_id"`
	Config          types.Object `tfsdk:"config"`
	Status          types.String `tfsdk:"status"`
	ConnectorName   types.String `tfsdk:"connector_name"`
	PublicationName types.String `tfsdk:"publication_name"`
	SlotName        types.String `tfsdk:"slot_name"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

// dataPipelineConfigModel maps the config { } block.
type dataPipelineConfigModel struct {
	DatabaseName types.String `tfsdk:"database_name"`
	Tables       types.List   `tfsdk:"tables"`
	TopicPrefix  types.String `tfsdk:"topic_prefix"`
	SnapshotMode types.String `tfsdk:"snapshot_mode"`
}

// NewDataPipelineResource returns a new dataPipelineResource factory.
func NewDataPipelineResource() resource.Resource {
	return &dataPipelineResource{}
}

func (r *dataPipelineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_pipeline"
}

func (r *dataPipelineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a data pipeline on the FoundryDB platform. Pipelines stream change data between managed services (e.g. PostgreSQL to Kafka via CDC). Provisioning is asynchronous; the provider polls until the pipeline reaches `Running` status. All creation arguments are immutable: any change destroys and recreates the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the pipeline.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the organization that owns this pipeline. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name for the pipeline. Must be unique within the organization. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pipeline_type": schema.StringAttribute{
				MarkdownDescription: "Pipeline variant. Currently only `cdc_pg_to_kafka` is supported. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the source managed service (e.g. the PostgreSQL cluster for CDC). Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"sink_service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the sink managed service (e.g. the Kafka cluster that receives CDC events). Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"config": schema.SingleNestedAttribute{
				MarkdownDescription: "Pipeline configuration block. All sub-fields are optional and immutable; changing any sub-field destroys and recreates the resource.",
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"database_name": schema.StringAttribute{
						MarkdownDescription: "Name of the source database from which CDC events are captured. Defaults to the managed service's primary database when omitted.",
						Optional:            true,
					},
					"tables": schema.ListAttribute{
						MarkdownDescription: "List of tables to include in CDC capture (e.g. `[\"public.orders\", \"public.users\"]`). When omitted, all tables in the source database are replicated.",
						Optional:            true,
						ElementType:         types.StringType,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"topic_prefix": schema.StringAttribute{
						MarkdownDescription: "Prefix applied to the Kafka topic name for each captured table. Defaults to the pipeline name when omitted.",
						Optional:            true,
					},
					"snapshot_mode": schema.StringAttribute{
						MarkdownDescription: "Debezium snapshot mode (e.g. `initial`, `never`, `always`). Defaults to `initial` when omitted.",
						Optional:            true,
					},
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current lifecycle status of the pipeline (e.g. `Provisioning`, `Running`, `Failed`).",
				Computed:            true,
			},
			"connector_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Kafka Connect connector backing this pipeline, populated after provisioning.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"publication_name": schema.StringAttribute{
				MarkdownDescription: "Name of the PostgreSQL publication created for CDC, populated after provisioning.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"slot_name": schema.StringAttribute{
				MarkdownDescription: "Name of the PostgreSQL replication slot created for CDC, populated after provisioning.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of when the pipeline was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of the last pipeline update.",
				Computed:            true,
			},
		},
	}
}

func (r *dataPipelineResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dataPipelineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dataPipelineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := foundrydb.CreateDataPipelineRequest{
		Name:            plan.Name.ValueString(),
		PipelineType:    foundrydb.DataPipelineType(plan.PipelineType.ValueString()),
		SourceServiceID: plan.SourceServiceID.ValueString(),
		SinkServiceID:   plan.SinkServiceID.ValueString(),
	}

	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		var cfgModel dataPipelineConfigModel
		resp.Diagnostics.Append(plan.Config.As(ctx, &cfgModel, basetypes_objectAsOptions())...)
		if resp.Diagnostics.HasError() {
			return
		}
		cfg := foundrydb.DataPipelineConfig{}
		if !cfgModel.DatabaseName.IsNull() && !cfgModel.DatabaseName.IsUnknown() {
			cfg.DatabaseName = cfgModel.DatabaseName.ValueString()
		}
		if !cfgModel.TopicPrefix.IsNull() && !cfgModel.TopicPrefix.IsUnknown() {
			cfg.TopicPrefix = cfgModel.TopicPrefix.ValueString()
		}
		if !cfgModel.SnapshotMode.IsNull() && !cfgModel.SnapshotMode.IsUnknown() {
			cfg.SnapshotMode = cfgModel.SnapshotMode.ValueString()
		}
		if !cfgModel.Tables.IsNull() && !cfgModel.Tables.IsUnknown() {
			var tables []string
			resp.Diagnostics.Append(cfgModel.Tables.ElementsAs(ctx, &tables, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			cfg.Tables = tables
		}
		createReq.Config = cfg
	}

	pipeline, err := r.client.CreateDataPipeline(ctx, plan.OrganizationID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating data pipeline", err.Error())
		return
	}

	pipeline, err = r.pollPipelineUntilRunning(ctx, plan.OrganizationID.ValueString(), pipeline.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for data pipeline to reach Running status", err.Error())
		return
	}

	resp.Diagnostics.Append(pipelineToState(ctx, pipeline, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dataPipelineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dataPipelineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pipeline, err := r.client.GetDataPipeline(ctx, state.OrganizationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading data pipeline", err.Error())
		return
	}
	if pipeline == nil {
		// Resource has been deleted outside of Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(pipelineToState(ctx, pipeline, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is intentionally not implemented: all data pipeline arguments are ForceNew.
// The framework will never call Update because every attribute change triggers
// a replacement. This method must still be present to satisfy resource.Resource.
func (r *dataPipelineResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Data pipeline update not supported",
		"All data pipeline attributes require replacement. This is a provider implementation error if this method is called.",
	)
}

func (r *dataPipelineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dataPipelineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteDataPipeline(ctx, state.OrganizationID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting data pipeline", err.Error())
	}
}

// ImportState implements resource.ResourceWithImportState. The import ID is
// "org_id/pipeline_id" (slash-separated).
func (r *dataPipelineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the format \"org_id/pipeline_id\", got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &dataPipelineResourceModel{
		OrganizationID: types.StringValue(parts[0]),
		ID:             types.StringValue(parts[1]),
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

// pollPipelineUntilRunning polls GetDataPipeline until the pipeline reaches
// Running status or a terminal failure. It times out after 15 minutes.
func (r *dataPipelineResource) pollPipelineUntilRunning(ctx context.Context, orgID, pipelineID string) (*foundrydb.DataPipeline, error) {
	const timeout = 15 * time.Minute
	const pollInterval = 10 * time.Second

	deadline := time.Now().Add(timeout)
	for {
		pipeline, err := r.client.GetDataPipeline(ctx, orgID, pipelineID)
		if err != nil {
			return nil, err
		}
		if pipeline == nil {
			return nil, fmt.Errorf("pipeline %q disappeared while waiting for it to reach Running status", pipelineID)
		}

		if pipeline.Status == "Running" {
			return pipeline, nil
		}
		if strings.Contains(pipeline.Status, "Failed") || strings.Contains(pipeline.Status, "Error") {
			msg := "unknown error"
			if pipeline.ErrorMessage != nil {
				msg = *pipeline.ErrorMessage
			}
			return nil, fmt.Errorf("pipeline %q reached terminal failure status %q: %s", pipelineID, pipeline.Status, msg)
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for pipeline %q to reach Running status (current status: %s)", pipelineID, pipeline.Status)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// pipelineToState maps an API DataPipeline into a Terraform state model.
func pipelineToState(ctx context.Context, p *foundrydb.DataPipeline, model *dataPipelineResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(p.ID)
	model.OrganizationID = types.StringValue(p.OrganizationID)
	model.Name = types.StringValue(p.Name)
	model.PipelineType = types.StringValue(string(p.PipelineType))
	model.SourceServiceID = types.StringValue(p.SourceServiceID)
	model.SinkServiceID = types.StringValue(p.SinkServiceID)
	model.Status = types.StringValue(p.Status)
	model.CreatedAt = types.StringValue(p.CreatedAt)
	model.UpdatedAt = types.StringValue(p.UpdatedAt)

	if p.ConnectorName != nil {
		model.ConnectorName = types.StringValue(*p.ConnectorName)
	} else {
		model.ConnectorName = types.StringNull()
	}
	if p.PublicationName != nil {
		model.PublicationName = types.StringValue(*p.PublicationName)
	} else {
		model.PublicationName = types.StringNull()
	}
	if p.SlotName != nil {
		model.SlotName = types.StringValue(*p.SlotName)
	} else {
		model.SlotName = types.StringNull()
	}

	// Build the config object from API response fields. When all sub-fields
	// are empty (the API did not echo them back), preserve the existing state
	// object rather than overwriting with nulls so Terraform does not see a
	// spurious diff on re-reads.
	hasConfigData := p.Config.DatabaseName != "" ||
		len(p.Config.Tables) > 0 ||
		p.Config.TopicPrefix != "" ||
		p.Config.SnapshotMode != ""

	if hasConfigData || model.Config.IsNull() || model.Config.IsUnknown() {
		tableElems := make([]attr.Value, len(p.Config.Tables))
		for i, t := range p.Config.Tables {
			tableElems[i] = types.StringValue(t)
		}
		tablesList, listDiags := types.ListValue(types.StringType, tableElems)
		diags.Append(listDiags...)
		if diags.HasError() {
			return diags
		}

		cfgAttrs := map[string]attr.Value{
			"database_name": types.StringValue(p.Config.DatabaseName),
			"tables":        tablesList,
			"topic_prefix":  types.StringValue(p.Config.TopicPrefix),
			"snapshot_mode": types.StringValue(p.Config.SnapshotMode),
		}
		cfgObj, objDiags := types.ObjectValue(dataPipelineConfigAttrTypes(), cfgAttrs)
		diags.Append(objDiags...)
		if diags.HasError() {
			return diags
		}
		model.Config = cfgObj
	}

	return diags
}

// dataPipelineConfigAttrTypes returns the attribute type map for the config object.
func dataPipelineConfigAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"database_name": types.StringType,
		"tables":        types.ListType{ElemType: types.StringType},
		"topic_prefix":  types.StringType,
		"snapshot_mode": types.StringType,
	}
}

// basetypes_objectAsOptions returns the ObjectAsOptions used when decoding
// types.Object into a struct. It is defined here to avoid importing
// types/basetypes directly in the function bodies above.
func basetypes_objectAsOptions() basetypes.ObjectAsOptions {
	return basetypes.ObjectAsOptions{}
}
