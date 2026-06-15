package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure queueResource satisfies resource.Resource.
var _ resource.Resource = &queueResource{}

// queueResource implements the foundrydb_queue resource.
type queueResource struct {
	client *foundrydb.Client
}

// queueResourceModel holds the Terraform state for a foundrydb_queue.
type queueResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	ServiceID                types.String `tfsdk:"service_id"`
	Name                     types.String `tfsdk:"name"`
	VisibilityTimeoutSeconds types.Int64  `tfsdk:"visibility_timeout_seconds"`
	MaxAttempts              types.Int64  `tfsdk:"max_attempts"`
	DLQEnabled               types.Bool   `tfsdk:"dlq_enabled"`
	Status                   types.String `tfsdk:"status"`
	DatabaseName             types.String `tfsdk:"database_name"`
}

// NewQueueResource returns a new queueResource factory.
func NewQueueResource() resource.Resource {
	return &queueResource{}
}

func (r *queueResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_queue"
}

func (r *queueResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a message queue on a FoundryDB PostgreSQL managed service. Queue state (messages) lives in the customer's database, transactional with their data. After creation, the provider polls until the queue reaches `Active` status. None of the creation arguments can be changed in-place; all modifications destroy and recreate the queue.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the queue.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the PostgreSQL managed service that hosts this queue. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique name for the queue within the service. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"visibility_timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Redelivery horizon in seconds: how long a claimed message stays invisible before a crashed consumer's claim expires and the message becomes visible again. Defaults to `30`. Changing this value destroys and recreates the resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_attempts": schema.Int64Attribute{
				MarkdownDescription: "Maximum delivery attempts before a message is dropped or dead-lettered (when `dlq_enabled` is `true`). Defaults to `5`. Changing this value destroys and recreates the resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"dlq_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether exhausted messages are moved to a dead-letter queue instead of being dropped. Defaults to `true`. Changing this value destroys and recreates the resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current lifecycle status of the queue: `Pending`, `Provisioning`, `Active`, `Deprovisioning`, or `Failed`.",
				Computed:            true,
			},
			"database_name": schema.StringAttribute{
				MarkdownDescription: "Name of the customer database on the PostgreSQL service where queue schema objects are created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *queueResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *queueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan queueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := foundrydb.QueueCreateRequest{
		Name: plan.Name.ValueString(),
	}

	if !plan.VisibilityTimeoutSeconds.IsNull() && !plan.VisibilityTimeoutSeconds.IsUnknown() {
		v := int(plan.VisibilityTimeoutSeconds.ValueInt64())
		createReq.VisibilityTimeoutSeconds = &v
	}
	if !plan.MaxAttempts.IsNull() && !plan.MaxAttempts.IsUnknown() {
		v := int(plan.MaxAttempts.ValueInt64())
		createReq.MaxAttempts = &v
	}
	if !plan.DLQEnabled.IsNull() && !plan.DLQEnabled.IsUnknown() {
		v := plan.DLQEnabled.ValueBool()
		createReq.DLQEnabled = &v
	}

	q, err := r.client.CreateQueue(ctx, plan.ServiceID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating queue", err.Error())
		return
	}

	// Poll until Active or Failed (provisioning is asynchronous).
	q, err = r.pollQueueUntilActive(ctx, plan.ServiceID.ValueString(), q.Name)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for queue to become active", err.Error())
		return
	}

	queueToState(q, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *queueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state queueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	q, err := r.client.GetQueue(ctx, state.ServiceID.ValueString(), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading queue", err.Error())
		return
	}
	if q == nil {
		// Resource has been deleted outside of Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	queueToState(q, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is intentionally not implemented: all queue arguments are ForceNew.
// The framework will never call Update because every attribute change triggers
// a replacement. This method must still be present to satisfy resource.Resource.
func (r *queueResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Queue update not supported",
		"All queue attributes require replacement. This is a provider implementation error if this method is called.",
	)
}

func (r *queueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state queueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteQueue(ctx, state.ServiceID.ValueString(), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting queue", err.Error())
		return
	}

	// Poll until the queue is gone (deprovisioning is asynchronous).
	if err := r.pollQueueUntilGone(ctx, state.ServiceID.ValueString(), state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error waiting for queue deletion to complete", err.Error())
	}
}

// pollQueueUntilActive polls GetQueue until the queue reaches Active status or
// fails. It times out after 10 minutes.
func (r *queueResource) pollQueueUntilActive(ctx context.Context, serviceID, queueName string) (*foundrydb.Queue, error) {
	const timeout = 10 * time.Minute
	const pollInterval = 5 * time.Second

	deadline := time.Now().Add(timeout)
	for {
		q, err := r.client.GetQueue(ctx, serviceID, queueName)
		if err != nil {
			return nil, err
		}
		if q == nil {
			return nil, fmt.Errorf("queue %q disappeared while waiting for it to become Active", queueName)
		}

		switch q.Status {
		case "Active":
			return q, nil
		case "Failed":
			msg := "unknown error"
			if q.ErrorMessage != nil {
				msg = *q.ErrorMessage
			}
			return nil, fmt.Errorf("queue %q provisioning failed: %s", queueName, msg)
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for queue %q to become Active (current status: %s)", queueName, q.Status)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// pollQueueUntilGone polls GetQueue until the queue returns nil (deleted) or
// times out after 10 minutes.
func (r *queueResource) pollQueueUntilGone(ctx context.Context, serviceID, queueName string) error {
	const timeout = 10 * time.Minute
	const pollInterval = 5 * time.Second

	deadline := time.Now().Add(timeout)
	for {
		q, err := r.client.GetQueue(ctx, serviceID, queueName)
		if err != nil {
			return err
		}
		if q == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for queue %q to be deleted (current status: %s)", queueName, q.Status)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// queueToState maps an API Queue into a Terraform state model.
func queueToState(q *foundrydb.Queue, model *queueResourceModel) {
	model.ID = types.StringValue(q.ID)
	model.ServiceID = types.StringValue(q.ServiceID)
	model.Name = types.StringValue(q.Name)
	model.VisibilityTimeoutSeconds = types.Int64Value(int64(q.VisibilityTimeoutSeconds))
	model.MaxAttempts = types.Int64Value(int64(q.MaxAttempts))
	model.DLQEnabled = types.BoolValue(q.DLQEnabled)
	model.Status = types.StringValue(q.Status)
	model.DatabaseName = types.StringValue(q.DatabaseName)
}
