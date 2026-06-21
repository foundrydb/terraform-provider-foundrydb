package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// validAttachmentKinds lists the companion-app kinds accepted by the platform.
var validAttachmentKinds = []string{
	"metabase",
	"directus",
	"hasura",
	"nocodb",
	"open-webui",
}

// Ensure attachmentResource satisfies the resource.Resource and
// resource.ResourceWithImportState interfaces.
var _ resource.Resource = &attachmentResource{}
var _ resource.ResourceWithImportState = &attachmentResource{}

// attachmentResource implements the foundrydb_attachment resource.
type attachmentResource struct {
	client *foundrydb.Client
}

// attachmentResourceModel holds the Terraform state for a foundrydb_attachment.
type attachmentResourceModel struct {
	// ID is the attachment ID returned in AttachmentSummary. It is the stable
	// key used to identify the attachment in subsequent list operations.
	ID             types.String `tfsdk:"id"`
	ParentServiceID types.String `tfsdk:"parent_service_id"`
	Kind           types.String `tfsdk:"kind"`
	PlanName       types.String `tfsdk:"plan_name"`
	Subdomain      types.String `tfsdk:"subdomain"`
	// Computed fields populated after provisioning.
	AppServiceID types.String `tfsdk:"app_service_id"`
	Name         types.String `tfsdk:"name"`
	Status       types.String `tfsdk:"status"`
	URL          types.String `tfsdk:"url"`
}

// NewAttachmentResource returns a new attachmentResource factory.
func NewAttachmentResource() resource.Resource {
	return &attachmentResource{}
}

func (r *attachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_attachment"
}

func (r *attachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a companion-app attachment on the FoundryDB platform. An attachment provisions a curated companion application (such as Metabase, Directus, Hasura, NocoDB, or Open WebUI) against a parent database service. The companion app is connected to the database over a private SDN and served at a subdomain with auto-TLS. After creation, the provider waits up to 20 minutes for the companion app to reach `running` status. All creation arguments are immutable: any change destroys and recreates the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier of the attachment, returned by the platform after creation.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"parent_service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the parent database service that the companion app is attached to. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Companion-app kind. One of: `metabase`, `directus`, `hasura`, `nocodb`, `open-webui`. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plan_name": schema.StringAttribute{
				MarkdownDescription: "Compute tier for the companion app (e.g. `tier-2`). When omitted, the catalog's default plan is used. Changing this value destroys and recreates the resource.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subdomain": schema.StringAttribute{
				MarkdownDescription: "Custom subdomain prefix for the companion app URL (e.g. `analytics` yields `analytics.foundrydb.com`). When omitted, a name is generated from the parent service and kind. Changing this value destroys and recreates the resource.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"app_service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the underlying app service created for the companion app. Use this ID to manage the lifecycle of the app service directly via `foundrydb_app_service` if needed.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the companion app service assigned by the platform.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current lifecycle status of the companion app (e.g. `running`, `provisioning`).",
				Computed:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "Public HTTPS URL of the companion app, populated after the app reaches `running` status.",
				Computed:            true,
			},
		},
	}
}

func (r *attachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *attachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan attachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	kind := plan.Kind.ValueString()
	if !isValidAttachmentKind(kind) {
		resp.Diagnostics.AddError(
			"Invalid attachment kind",
			fmt.Sprintf("kind must be one of [%s], got %q.", strings.Join(validAttachmentKinds, ", "), kind),
		)
		return
	}

	createReq := foundrydb.CreateAttachmentRequest{
		Kind: kind,
	}
	if !plan.PlanName.IsNull() && !plan.PlanName.IsUnknown() {
		createReq.PlanName = plan.PlanName.ValueString()
	}
	if !plan.Subdomain.IsNull() && !plan.Subdomain.IsUnknown() {
		createReq.Subdomain = plan.Subdomain.ValueString()
	}

	appSvc, err := r.client.CreateAttachment(ctx, plan.ParentServiceID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating attachment", err.Error())
		return
	}

	// Wait for the companion app to reach running status before returning.
	appSvc, err = r.client.WaitForAppRunning(ctx, appSvc.ID, 20*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for attachment companion app to become running", err.Error())
		return
	}

	// Resolve the attachment ID and URL by listing the parent's attachments.
	summaries, err := r.client.ListAttachments(ctx, plan.ParentServiceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing attachments after create", err.Error())
		return
	}

	attachmentToState(appSvc, summaries, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *attachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state attachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the underlying app service for status and URL.
	appSvc, err := r.client.GetAppService(ctx, state.AppServiceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading attachment app service", err.Error())
		return
	}
	if appSvc == nil {
		// The companion app service was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	// Refresh URL and wiring status from the parent's attachment list.
	summaries, err := r.client.ListAttachments(ctx, state.ParentServiceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing attachments on read", err.Error())
		return
	}

	attachmentToState(appSvc, summaries, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is intentionally not implemented: all attachment arguments are ForceNew.
// The framework will never call Update because every attribute change triggers
// a replacement. This method must still be present to satisfy resource.Resource.
func (r *attachmentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Attachment update not supported",
		"All attachment attributes require replacement. This is a provider implementation error if this method is called.",
	)
}

func (r *attachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state attachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Deleting the companion app service removes the attachment wiring as well.
	if err := r.client.DeleteAppService(ctx, state.AppServiceID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting attachment companion app", err.Error())
	}
}

// ImportState implements resource.ResourceWithImportState. The import ID is
// "parent_service_id/app_service_id" (slash-separated).
func (r *attachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the format \"parent_service_id/app_service_id\", got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &attachmentResourceModel{
		ParentServiceID: types.StringValue(parts[0]),
		AppServiceID:    types.StringValue(parts[1]),
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

// attachmentToState maps an AppService and its AttachmentSummary list into a
// Terraform state model. The attachment ID and URL are resolved from the summary
// whose AppServiceID matches the created companion app.
func attachmentToState(appSvc *foundrydb.AppService, summaries []foundrydb.AttachmentSummary, model *attachmentResourceModel) {
	model.AppServiceID = types.StringValue(appSvc.ID)
	model.Name = types.StringValue(appSvc.Name)
	model.Status = types.StringValue(appSvc.Status)

	// Locate the matching summary for attachment ID and URL.
	for _, s := range summaries {
		if s.AppServiceID == appSvc.ID {
			model.ID = types.StringValue(s.AttachmentID)
			model.Kind = types.StringValue(s.Kind)
			if s.URL != "" {
				model.URL = types.StringValue(s.URL)
			} else {
				model.URL = types.StringNull()
			}
			return
		}
	}

	// No matching summary: preserve existing ID and URL from state.
	if model.ID.IsNull() || model.ID.IsUnknown() {
		model.ID = types.StringValue(appSvc.ID)
	}
	model.URL = types.StringNull()
}

// isValidAttachmentKind reports whether kind is one of the accepted companion-app kinds.
func isValidAttachmentKind(kind string) bool {
	for _, k := range validAttachmentKinds {
		if k == kind {
			return true
		}
	}
	return false
}
