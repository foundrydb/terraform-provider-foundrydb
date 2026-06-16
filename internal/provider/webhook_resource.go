package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure webhookResource satisfies the resource.Resource and
// resource.ResourceWithImportState interfaces.
var _ resource.Resource = &webhookResource{}
var _ resource.ResourceWithImportState = &webhookResource{}

// webhookResource implements the foundrydb_webhook resource.
type webhookResource struct {
	client *foundrydb.Client
}

// webhookResourceModel holds the Terraform state for a foundrydb_webhook.
type webhookResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	URL            types.String `tfsdk:"url"`
	Events         types.List   `tfsdk:"events"`
	Active         types.Bool   `tfsdk:"active"`
	Secret         types.String `tfsdk:"secret"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

// NewWebhookResource returns a new webhookResource factory.
func NewWebhookResource() resource.Resource {
	return &webhookResource{}
}

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a webhook endpoint for a FoundryDB organization. The endpoint secret is returned only at creation time and stored in Terraform state; it is never returned by subsequent API reads. All attributes are immutable: any change destroys and recreates the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier of the webhook endpoint.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "ID of the organization that owns this webhook. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "HTTPS URL that receives webhook event payloads. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"events": schema.ListAttribute{
				MarkdownDescription: "List of event types to subscribe to (e.g. `[\"service.created\", \"service.deleted\"]`). An empty list subscribes to all events. Changing this value destroys and recreates the resource.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the webhook is currently active. Managed server-side: the platform disables the endpoint after repeated delivery failures.",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "HMAC signing secret used to verify webhook payloads. Returned only at creation time; preserved in state thereafter. Never returned by subsequent API reads.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of when the webhook endpoint was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of when the webhook endpoint was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var events []string
	if !plan.Events.IsNull() && !plan.Events.IsUnknown() {
		resp.Diagnostics.Append(plan.Events.ElementsAs(ctx, &events, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	createReq := foundrydb.CreateWebhookRequest{
		URL:    plan.URL.ValueString(),
		Events: events,
	}

	wh, err := r.client.CreateOrgWebhook(ctx, plan.OrganizationID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating webhook", err.Error())
		return
	}

	// The secret is returned only on Create; capture it before mapping the rest of the state.
	secret := wh.Secret

	webhookToState(ctx, wh, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve the one-time secret; webhookToState leaves it empty because the
	// Read path never has it.
	plan.Secret = types.StringValue(secret)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wh, err := r.client.GetOrgWebhook(ctx, state.OrganizationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading webhook", err.Error())
		return
	}
	if wh == nil {
		// Resource has been deleted outside of Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	// Preserve the secret from prior state: the API never returns it after creation.
	preservedSecret := state.Secret

	webhookToState(ctx, wh, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Secret = preservedSecret

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is not implemented: all webhook attributes require replacement.
// The framework will never call Update because every attribute is ForceNew.
// This method is present only to satisfy the resource.Resource interface.
func (r *webhookResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Webhook update not supported",
		"All webhook attributes require replacement. This is a provider implementation error if this method is called.",
	)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteOrgWebhook(ctx, state.OrganizationID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting webhook", err.Error())
	}
}

// ImportState implements resource.ResourceWithImportState. The import ID must
// be in the format "org_id/webhook_id".
func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the format \"org_id/webhook_id\", got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &webhookResourceModel{
		OrganizationID: types.StringValue(parts[0]),
		ID:             types.StringValue(parts[1]),
	})...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call Read to populate all remaining attributes. The secret will be null
	// after import because the API never returns it; this is expected and
	// documented behaviour.
	readReq := resource.ReadRequest{State: tfsdk.State(resp.State)}
	readResp := &resource.ReadResponse{State: resp.State}
	r.Read(ctx, readReq, readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
	resp.State = readResp.State
}

// webhookToState maps an API WebhookEndpoint into a Terraform state model.
// It does NOT set the secret field; callers must handle that separately because
// the API only returns the secret at creation time.
func webhookToState(_ context.Context, wh *foundrydb.WebhookEndpoint, model *webhookResourceModel, d *diag.Diagnostics) {
	model.ID = types.StringValue(wh.ID)
	model.URL = types.StringValue(wh.URL)
	model.Active = types.BoolValue(wh.Active)
	model.CreatedAt = types.StringValue(wh.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	model.UpdatedAt = types.StringValue(wh.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))

	eventElems := make([]attr.Value, len(wh.Events))
	for i, e := range wh.Events {
		eventElems[i] = types.StringValue(e)
	}
	var listDiags diag.Diagnostics
	model.Events, listDiags = types.ListValue(types.StringType, eventElems)
	d.Append(listDiags...)
}
