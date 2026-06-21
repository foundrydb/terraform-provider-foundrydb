package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure stackResource satisfies the resource.Resource and
// resource.ResourceWithImportState interfaces.
var _ resource.Resource = &stackResource{}
var _ resource.ResourceWithImportState = &stackResource{}

// stackProvisionTimeout is the maximum time the provider waits for a stack to
// reach Running status after launch. Stacks provision real VMs and wire
// multiple platform primitives; 20 minutes is a generous but realistic ceiling.
const stackProvisionTimeout = 20 * time.Minute

// stackResource implements the foundrydb_stack resource.
type stackResource struct {
	client *foundrydb.Client
}

// stackResourceModel holds the Terraform state for a foundrydb_stack.
type stackResourceModel struct {
	ID                   types.String  `tfsdk:"id"`
	Name                 types.String  `tfsdk:"name"`
	TemplateName         types.String  `tfsdk:"template_name"`
	TemplateID           types.String  `tfsdk:"template_id"`
	OrganizationID       types.String  `tfsdk:"organization_id"`
	AcceptedMonthlyCost  types.Float64 `tfsdk:"accepted_monthly_cost"`
	Status               types.String  `tfsdk:"status"`
	EndpointURL          types.String  `tfsdk:"endpoint_url"`
	EstimatedMonthlyCost types.Float64 `tfsdk:"estimated_monthly_cost"`
	Resources            types.List    `tfsdk:"resources"`
}

// stackResourceItemModel represents one child resource of a stack in state.
type stackResourceItemModel struct {
	SymbolicName types.String `tfsdk:"symbolic_name"`
	Kind         types.String `tfsdk:"kind"`
	Status       types.String `tfsdk:"status"`
	ServiceID    types.String `tfsdk:"service_id"`
}

// stackResourceItemAttrTypes is the attr.Type map for a stack resource item object.
var stackResourceItemAttrTypes = map[string]attr.Type{
	"symbolic_name": types.StringType,
	"kind":          types.StringType,
	"status":        types.StringType,
	"service_id":    types.StringType,
}

// NewStackResource returns a new stackResource factory.
func NewStackResource() resource.Resource {
	return &stackResource{}
}

func (r *stackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack"
}

func (r *stackResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Launches and manages a FoundryDB vertical-starter stack. A stack provisions a set of platform primitives (database, file storage, inference, app service) from a first-party catalog template in a single atomic operation. After creation, the provider waits up to 20 minutes for the stack to reach `Running` status. All input fields except `organization_id` are immutable after launch; changing `name` or `template_name` destroys and recreates the stack.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the stack.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name for this stack instance. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"template_name": schema.StringAttribute{
				MarkdownDescription: "Name of the first-party catalog template to launch (e.g. `rag-chatbot`). Use the `foundrydb_stack_templates` data source to list available templates. Exactly one of `template_name` or `template_id` must be set. Changing this value destroys and recreates the resource.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"template_id": schema.StringAttribute{
				MarkdownDescription: "UUID of a customer-authored marketplace template to launch. The template must be visible to the caller (publicly published, or `org_shared`/`private` within the caller's organization). Exactly one of `template_name` or `template_id` must be set. Manage the template itself with `foundrydb_stack_template`. Changing this value destroys and recreates the resource.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Optional organization UUID to scope this stack and its inference key to. When omitted, the caller's primary billing organization is used. Changing this value destroys and recreates the resource.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"accepted_monthly_cost": schema.Float64Attribute{
				MarkdownDescription: "The estimated monthly cost in USD that the operator explicitly accepted before launching. Read the `monthly_total` from the `foundrydb_stack_templates` data source and pass it here. The launch is rejected if the freshly computed estimate differs from this value by more than $0.01. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.Float64{
					float64planmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current lifecycle status of the stack (e.g. `Running`, `Provisioning`, `Failed`).",
				Computed:            true,
			},
			"endpoint_url": schema.StringAttribute{
				MarkdownDescription: "Public URL of the stack's primary application endpoint. Populated once the stack reaches `Running` status.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"estimated_monthly_cost": schema.Float64Attribute{
				MarkdownDescription: "Actual estimated monthly cost in USD as computed by the platform at launch time.",
				Computed:            true,
				PlanModifiers: []planmodifier.Float64{
					float64planmodifier.UseStateForUnknown(),
				},
			},
			"resources": schema.ListNestedAttribute{
				MarkdownDescription: "Child resources provisioned by this stack. Each entry corresponds to one platform primitive declared in the template.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"symbolic_name": schema.StringAttribute{
							MarkdownDescription: "Symbolic name of the resource as declared in the template descriptor (e.g. `db`, `files`, `app`).",
							Computed:            true,
						},
						"kind": schema.StringAttribute{
							MarkdownDescription: "Platform primitive kind: `database`, `files`, `inference`, or `app`.",
							Computed:            true,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "Provisioning status of this individual resource.",
							Computed:            true,
						},
						"service_id": schema.StringAttribute{
							MarkdownDescription: "UUID of the backing service for service-backed kinds (`database`, `files`, `app`). Empty for org-scoped resources such as inference keys.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (r *stackResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	switch v := req.ProviderData.(type) {
	case *providerData:
		r.client = v.client
	case *foundrydb.Client:
		r.client = v
	default:
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *providerData, got %T", req.ProviderData),
		)
	}
}

func (r *stackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan stackResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that exactly one of template_name / template_id is set.
	hasTemplateName := !plan.TemplateName.IsNull() && !plan.TemplateName.IsUnknown() && plan.TemplateName.ValueString() != ""
	hasTemplateID := !plan.TemplateID.IsNull() && !plan.TemplateID.IsUnknown() && plan.TemplateID.ValueString() != ""
	if !hasTemplateName && !hasTemplateID {
		resp.Diagnostics.AddError(
			"Missing template selector",
			"Exactly one of template_name or template_id must be set.",
		)
		return
	}
	if hasTemplateName && hasTemplateID {
		resp.Diagnostics.AddError(
			"Conflicting template selectors",
			"Only one of template_name or template_id may be set, not both.",
		)
		return
	}

	cost := plan.AcceptedMonthlyCost.ValueFloat64()
	launchReq := foundrydb.StackLaunchRequest{
		Name:                plan.Name.ValueString(),
		AcceptedMonthlyCost: &cost,
	}
	if hasTemplateName {
		launchReq.TemplateName = plan.TemplateName.ValueString()
	}
	if hasTemplateID {
		launchReq.TemplateID = plan.TemplateID.ValueString()
	}
	if !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() {
		launchReq.OrganizationID = plan.OrganizationID.ValueString()
	}

	stack, err := r.client.LaunchStack(ctx, launchReq)
	if err != nil {
		resp.Diagnostics.AddError("Error launching stack", err.Error())
		return
	}

	// Wait for the stack to reach Running status. Stacks compose multiple child
	// resources (VMs, file services, inference keys, app services) and can take
	// upward of 15 minutes on first launch.
	stack, err = r.client.WaitForStackRunning(ctx, stack.ID, stackProvisionTimeout)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for stack to reach Running status", err.Error())
		return
	}

	stackToState(stack, &plan, resp.Diagnostics.AddError)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *stackResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state stackResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stack, err := r.client.GetStack(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading stack", err.Error())
		return
	}
	if stack == nil || strings.EqualFold(stack.Status, "deleted") {
		// Stack no longer exists; remove from state so Terraform knows to recreate it.
		resp.State.RemoveResource(ctx)
		return
	}

	stackToState(stack, &state, resp.Diagnostics.AddError)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is a no-op because every meaningful field carries RequiresReplace.
// Terraform will never call Update for this resource; it will always destroy
// and recreate on any change. The method is required to satisfy the
// resource.Resource interface.
func (r *stackResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

func (r *stackResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state stackResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteStack(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting stack", err.Error())
		return
	}

	// Poll until the stack is gone or reaches Deleted status. The reconciler
	// tears down child resources before the stack record itself settles.
	deadline := time.Now().Add(stackProvisionTimeout)
	for {
		stack, err := r.client.GetStack(ctx, state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error polling stack during deletion", err.Error())
			return
		}
		// nil means 404 (already gone): deletion complete.
		if stack == nil || strings.EqualFold(stack.Status, "deleted") {
			return
		}
		if time.Now().After(deadline) {
			resp.Diagnostics.AddError(
				"Timeout waiting for stack deletion",
				fmt.Sprintf("Stack %s did not reach Deleted status within %s (current: %s)", state.ID.ValueString(), stackProvisionTimeout, stack.Status),
			)
			return
		}
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Context cancelled while waiting for stack deletion", ctx.Err().Error())
			return
		case <-time.After(10 * time.Second):
		}
	}
}

// ImportState implements resource.ResourceWithImportState. The import ID is the stack UUID.
func (r *stackResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.Set(ctx, &stackResourceModel{
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

// stackToState maps an API Stack into a Terraform state model.
// addError is a callback matching the signature of resp.Diagnostics.AddError.
func stackToState(stack *foundrydb.Stack, model *stackResourceModel, addError func(summary, detail string)) {
	model.ID = types.StringValue(stack.ID)
	model.Name = types.StringValue(stack.Name)
	model.Status = types.StringValue(stack.Status)
	model.EndpointURL = types.StringValue(stack.EndpointURL)
	model.EstimatedMonthlyCost = types.Float64Value(stack.EstimatedMonthlyCost)

	// Populate the template selector fields from the API response. The API
	// echoes TemplateName for first-party stacks and SourceTemplateID for
	// marketplace stacks. Preserve whichever was in state for the other field
	// so plan/state stays consistent.
	if stack.TemplateName != "" {
		model.TemplateName = types.StringValue(stack.TemplateName)
	}
	if stack.SourceTemplateID != "" {
		model.TemplateID = types.StringValue(stack.SourceTemplateID)
	}

	// organization_id is not returned by the API; preserve whatever was set in state/plan.

	items := make([]attr.Value, len(stack.Resources))
	for i, res := range stack.Resources {
		obj, diags := types.ObjectValue(stackResourceItemAttrTypes, map[string]attr.Value{
			"symbolic_name": types.StringValue(res.SymbolicName),
			"kind":          types.StringValue(res.Kind),
			"status":        types.StringValue(res.Status),
			"service_id":    types.StringValue(res.ServiceID),
		})
		if diags.HasError() {
			for _, d := range diags {
				addError("Error building stack resource item", d.Detail())
			}
			return
		}
		items[i] = obj
	}

	listVal, diags := types.ListValue(types.ObjectType{AttrTypes: stackResourceItemAttrTypes}, items)
	if diags.HasError() {
		for _, d := range diags {
			addError("Error building stack resources list", d.Detail())
		}
		return
	}
	model.Resources = listVal
}
