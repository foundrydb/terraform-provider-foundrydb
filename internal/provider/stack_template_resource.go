package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure stackTemplateResource satisfies the resource.Resource and
// resource.ResourceWithImportState interfaces.
var _ resource.Resource = &stackTemplateResource{}
var _ resource.ResourceWithImportState = &stackTemplateResource{}

// stackTemplateResource implements the foundrydb_stack_template resource.
type stackTemplateResource struct {
	client *foundrydb.Client
}

// stackTemplateResourceModel holds the Terraform state for a
// foundrydb_stack_template.
type stackTemplateResourceModel struct {
	// Input attributes
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	DisplayName  types.String `tfsdk:"display_name"`
	Description  types.String `tfsdk:"description"`
	Version      types.String `tfsdk:"version"`
	Visibility   types.String `tfsdk:"visibility"`
	Descriptor   types.String `tfsdk:"descriptor"`
	Publish      types.Bool   `tfsdk:"publish"`
	// Computed attributes
	PublicationStatus types.String `tfsdk:"publication_status"`
	OrganizationID    types.String `tfsdk:"organization_id"`
}

// NewStackTemplateResource returns a new stackTemplateResource factory.
func NewStackTemplateResource() resource.Resource {
	return &stackTemplateResource{}
}

func (r *stackTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack_template"
}

func (r *stackTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a customer-authored stack template in the FoundryDB marketplace. A custom template contains a `StackDescriptor` that declares which platform primitives to compose (databases, file services, app services, inference keys) and how they depend on each other. Templates start in `draft` status and are only visible within the owning organization. Set `visibility` to `org_shared` or `public` and set `publish = true` to share the template. Published templates are immutable: editing a published template requires unpublishing it first or creating a new version.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the custom template.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique slug-style identifier for this template (e.g. `my-rag-stack`). Used as a stable key for the template in the marketplace. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name shown in the marketplace catalog (e.g. `My RAG Stack`). Can be updated while the template is in draft, rejected, or unpublished state.",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Short description of what this template provisions. Can be updated while the template is in draft, rejected, or unpublished state.",
				Optional:            true,
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Semantic version of this template descriptor (e.g. `1.0.0`). Defaults to `1.0.0` when omitted. Can be updated while the template is in draft, rejected, or unpublished state.",
				Optional:            true,
				Computed:            true,
			},
			"visibility": schema.StringAttribute{
				MarkdownDescription: "Who can see and launch this template. One of `private` (owning org only), `org_shared` (all members of the owning org), or `public` (all organizations in the marketplace, subject to platform review). Can be updated while the template is in draft, rejected, or unpublished state.",
				Optional:            true,
				Computed:            true,
			},
			"descriptor": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded `StackDescriptor` that declares the resources to compose and their dependencies. The descriptor is validated by the platform on create and update. Can be updated while the template is in draft, rejected, or unpublished state.",
				Required:            true,
			},
			"publish": schema.BoolAttribute{
				MarkdownDescription: "When `true`, the provider calls `PublishStackTemplate` immediately after create (or after the first update that sets this to `true`). For `org_shared` visibility, this publishes immediately. For `public` visibility, this submits the template to the platform moderation queue. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"publication_status": schema.StringAttribute{
				MarkdownDescription: "Current moderation lifecycle of the template. One of `draft`, `submitted`, `approved`, `published`, `rejected`, or `unpublished`.",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the organization that owns this template. Populated by the platform from the caller's primary billing organization.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *stackTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *stackTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan stackTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	descriptor, err := parseDescriptor(plan.Descriptor.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid descriptor JSON", err.Error())
		return
	}

	createReq := foundrydb.CustomTemplateRequest{
		Name:       plan.Name.ValueString(),
		Descriptor: descriptor,
	}
	if !plan.DisplayName.IsNull() && !plan.DisplayName.IsUnknown() {
		createReq.DisplayName = plan.DisplayName.ValueString()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		createReq.Description = plan.Description.ValueString()
	}
	if !plan.Version.IsNull() && !plan.Version.IsUnknown() {
		createReq.Version = plan.Version.ValueString()
	}
	if !plan.Visibility.IsNull() && !plan.Visibility.IsUnknown() {
		createReq.Visibility = foundrydb.StackVisibility(plan.Visibility.ValueString())
	}

	tmpl, err := r.client.CreateStackTemplate(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating stack template", err.Error())
		return
	}

	// Optionally publish immediately after creation.
	if !plan.Publish.IsNull() && !plan.Publish.IsUnknown() && plan.Publish.ValueBool() {
		published, err := r.client.PublishStackTemplate(ctx, tmpl.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error publishing stack template after create", err.Error())
			return
		}
		tmpl = published
	}

	stackTemplateToState(tmpl, plan.Descriptor.ValueString(), plan.Publish, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *stackTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state stackTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tmpl, err := r.client.GetStackTemplate(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading stack template", err.Error())
		return
	}
	if tmpl == nil {
		// Template was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	// Re-serialize the descriptor so the attribute stays in sync with the platform.
	descriptorJSON, err := json.Marshal(tmpl.Descriptor)
	if err != nil {
		resp.Diagnostics.AddError("Error serializing descriptor from API response", err.Error())
		return
	}

	stackTemplateToState(tmpl, string(descriptorJSON), state.Publish, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *stackTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan stackTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state stackTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	descriptor, err := parseDescriptor(plan.Descriptor.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid descriptor JSON", err.Error())
		return
	}

	updateReq := foundrydb.CustomTemplateRequest{
		Descriptor: descriptor,
	}
	if !plan.DisplayName.IsNull() && !plan.DisplayName.IsUnknown() {
		updateReq.DisplayName = plan.DisplayName.ValueString()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		updateReq.Description = plan.Description.ValueString()
	}
	if !plan.Version.IsNull() && !plan.Version.IsUnknown() {
		updateReq.Version = plan.Version.ValueString()
	}
	if !plan.Visibility.IsNull() && !plan.Visibility.IsUnknown() {
		updateReq.Visibility = foundrydb.StackVisibility(plan.Visibility.ValueString())
	}

	tmpl, err := r.client.UpdateStackTemplate(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating stack template", err.Error())
		return
	}

	// If publish was toggled on in this update, initiate publication.
	if !plan.Publish.IsNull() && !plan.Publish.IsUnknown() && plan.Publish.ValueBool() &&
		(state.Publish.IsNull() || state.Publish.IsUnknown() || !state.Publish.ValueBool()) {
		published, err := r.client.PublishStackTemplate(ctx, tmpl.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error publishing stack template on update", err.Error())
			return
		}
		tmpl = published
	}

	stackTemplateToState(tmpl, plan.Descriptor.ValueString(), plan.Publish, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *stackTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state stackTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteStackTemplate(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting stack template", err.Error())
	}
}

// ImportState implements resource.ResourceWithImportState. The import ID is the
// template UUID.
func (r *stackTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.Set(ctx, &stackTemplateResourceModel{
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

// stackTemplateToState maps an API CustomStackTemplate into a Terraform state
// model. descriptorJSON is the raw JSON string to store (either from the plan
// or re-serialized from the API); publish is preserved from plan/state since
// the API does not return the boolean input.
func stackTemplateToState(tmpl *foundrydb.CustomStackTemplate, descriptorJSON string, publish types.Bool, model *stackTemplateResourceModel) {
	model.ID = types.StringValue(tmpl.ID)
	model.Name = types.StringValue(tmpl.Name)
	model.DisplayName = types.StringValue(tmpl.DisplayName)
	model.Description = types.StringValue(tmpl.Description)
	model.Version = types.StringValue(tmpl.Version)
	model.Visibility = types.StringValue(string(tmpl.Visibility))
	model.PublicationStatus = types.StringValue(string(tmpl.PublicationStatus))
	model.OrganizationID = types.StringValue(tmpl.OrganizationID)
	model.Descriptor = types.StringValue(descriptorJSON)

	// publish is a write-only intent flag; carry it forward unchanged.
	if publish.IsNull() || publish.IsUnknown() {
		model.Publish = types.BoolValue(false)
	} else {
		model.Publish = publish
	}
}

// parseDescriptor unmarshals a JSON string into a StackDescriptor.
// Returns a descriptive error when the string is not valid JSON or does not
// match the expected structure.
func parseDescriptor(raw string) (foundrydb.StackDescriptor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return foundrydb.StackDescriptor{}, fmt.Errorf("descriptor must be a non-empty JSON object")
	}
	var d foundrydb.StackDescriptor
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return foundrydb.StackDescriptor{}, fmt.Errorf("descriptor is not valid JSON: %w", err)
	}
	return d, nil
}
