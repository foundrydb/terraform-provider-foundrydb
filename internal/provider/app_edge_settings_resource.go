package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure appEdgeSettingsResource satisfies resource.Resource.
var _ resource.Resource = &appEdgeSettingsResource{}

// appEdgeSettingsResource implements the foundrydb_app_edge_settings resource.
type appEdgeSettingsResource struct {
	edge *edgeClient
}

// cacheRuleModel is the Terraform representation of one EdgeCacheRule.
type cacheRuleModel struct {
	PathPrefix types.String `tfsdk:"path_prefix"`
	TTLSeconds types.Int64  `tfsdk:"ttl_seconds"`
}

// rateLimitModel is the Terraform representation of EdgeRateLimit.
type rateLimitModel struct {
	RequestsPerSecond types.Int64  `tfsdk:"requests_per_second"`
	Burst             types.Int64  `tfsdk:"burst"`
	Key               types.String `tfsdk:"key"`
}

// appEdgeSettingsResourceModel holds the Terraform state for a foundrydb_app_edge_settings.
type appEdgeSettingsResourceModel struct {
	ID            types.String `tfsdk:"id"`
	AppServiceID  types.String `tfsdk:"app_service_id"`
	WAFMode       types.String `tfsdk:"waf_mode"`
	CacheRules    types.List   `tfsdk:"cache_rules"`
	RateLimit     types.List   `tfsdk:"rate_limit"`
	ConfigVersion types.Int64  `tfsdk:"config_version"`
}

// cacheRuleAttrTypes is the attribute type map for the cache_rules list elements.
var cacheRuleAttrTypes = map[string]attr.Type{
	"path_prefix": types.StringType,
	"ttl_seconds": types.Int64Type,
}

// rateLimitAttrTypes is the attribute type map for the rate_limit list elements.
var rateLimitAttrTypes = map[string]attr.Type{
	"requests_per_second": types.Int64Type,
	"burst":               types.Int64Type,
	"key":                 types.StringType,
}

// cacheRuleObjectType is the tftypes object type for a single cache rule.
var cacheRuleObjectType = types.ObjectType{AttrTypes: cacheRuleAttrTypes}

// rateLimitObjectType is the tftypes object type for a single rate limit block.
var rateLimitObjectType = types.ObjectType{AttrTypes: rateLimitAttrTypes}

// NewAppEdgeSettingsResource returns a new appEdgeSettingsResource factory.
func NewAppEdgeSettingsResource() resource.Resource {
	return &appEdgeSettingsResource{}
}

func (r *appEdgeSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_edge_settings"
}

func (r *appEdgeSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the edge settings (cache rules, rate limiting, and WAF mode) for a FoundryDB app service. The settings are applied to the edge fleet via a single PUT call; the resource replaces the entire settings object on every apply. Deleting this resource resets the settings to their defaults (empty cache rules, no rate limit, WAF mode `off`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Always equal to `app_service_id`. Used as the Terraform resource identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the app service whose edge settings this resource manages. Changing this value destroys and recreates the resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"waf_mode": schema.StringAttribute{
				MarkdownDescription: "Web application firewall mode: `off` (disabled) or `detect` (log without blocking). Defaults to `off`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cache_rules": schema.ListNestedAttribute{
				MarkdownDescription: "Ordered list of cache rules. Each rule matches requests whose path begins with `path_prefix` and caches the response for `ttl_seconds`. Rules are evaluated in order; the first match wins.",
				Optional:            true,
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"path_prefix": schema.StringAttribute{
							MarkdownDescription: "URL path prefix to match (e.g. `/static`). Must start with `/`.",
							Required:            true,
						},
						"ttl_seconds": schema.Int64Attribute{
							MarkdownDescription: "Time-to-live in seconds for matched responses.",
							Required:            true,
						},
					},
				},
			},
			"rate_limit": schema.ListNestedAttribute{
				MarkdownDescription: "Optional rate limit configuration. When present, exactly one block is expected. When absent, rate limiting is disabled.",
				Optional:            true,
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"requests_per_second": schema.Int64Attribute{
							MarkdownDescription: "Sustained request rate allowed per key per second.",
							Required:            true,
						},
						"burst": schema.Int64Attribute{
							MarkdownDescription: "Maximum burst size (token bucket capacity).",
							Required:            true,
						},
						"key": schema.StringAttribute{
							MarkdownDescription: "Key used to bucket requests: `ip` (client IP) or `api_key` (API key header).",
							Required:            true,
						},
					},
				},
			},
			"config_version": schema.Int64Attribute{
				MarkdownDescription: "Monotonically increasing version number of the edge configuration. Increases each time settings are updated.",
				Computed:            true,
			},
		},
	}
}

func (r *appEdgeSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *appEdgeSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appEdgeSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := buildEdgeSettingsRequest(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.edge.UpdateEdgeSettings(ctx, plan.AppServiceID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error applying edge settings", err.Error())
		return
	}

	resp.Diagnostics.Append(edgeSettingsToState(ctx, settings, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *appEdgeSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appEdgeSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	edgeStatus, err := r.edge.GetEdgeStatus(ctx, state.AppServiceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading edge settings", err.Error())
		return
	}

	// When the edge tier has not recorded any settings yet, keep existing state.
	if edgeStatus.Settings == nil {
		return
	}

	resp.Diagnostics.Append(edgeSettingsToState(ctx, edgeStatus.Settings, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *appEdgeSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan appEdgeSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state appEdgeSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := buildEdgeSettingsRequest(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.edge.UpdateEdgeSettings(ctx, state.AppServiceID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating edge settings", err.Error())
		return
	}

	resp.Diagnostics.Append(edgeSettingsToState(ctx, settings, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete resets the edge settings to defaults by sending an empty settings object.
func (r *appEdgeSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appEdgeSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wafOff := "off"
	_, err := r.edge.UpdateEdgeSettings(ctx, state.AppServiceID.ValueString(), EdgeSettingsRequest{
		CacheRules: []EdgeCacheRule{},
		WAFMode:    &wafOff,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error resetting edge settings on delete", err.Error())
	}
}

// buildEdgeSettingsRequest converts a Terraform plan model into an API request.
func buildEdgeSettingsRequest(ctx context.Context, model *appEdgeSettingsResourceModel) (EdgeSettingsRequest, fwdiag.Diagnostics) {
	var diags fwdiag.Diagnostics
	req := EdgeSettingsRequest{}

	// WAF mode.
	if !model.WAFMode.IsNull() && !model.WAFMode.IsUnknown() {
		mode := model.WAFMode.ValueString()
		req.WAFMode = &mode
	}

	// Cache rules.
	if !model.CacheRules.IsNull() && !model.CacheRules.IsUnknown() {
		var rules []cacheRuleModel
		d := model.CacheRules.ElementsAs(ctx, &rules, false)
		diags.Append(d...)
		if diags.HasError() {
			return req, diags
		}
		apiRules := make([]EdgeCacheRule, len(rules))
		for i, rule := range rules {
			apiRules[i] = EdgeCacheRule{
				PathPrefix: rule.PathPrefix.ValueString(),
				TTLSeconds: int(rule.TTLSeconds.ValueInt64()),
			}
		}
		req.CacheRules = apiRules
	}

	// Rate limit (at most one block).
	if !model.RateLimit.IsNull() && !model.RateLimit.IsUnknown() {
		var limits []rateLimitModel
		d := model.RateLimit.ElementsAs(ctx, &limits, false)
		diags.Append(d...)
		if diags.HasError() {
			return req, diags
		}
		if len(limits) > 0 {
			req.RateLimit = &EdgeRateLimit{
				RequestsPerSecond: int(limits[0].RequestsPerSecond.ValueInt64()),
				Burst:             int(limits[0].Burst.ValueInt64()),
				Key:               limits[0].Key.ValueString(),
			}
		}
	}

	return req, diags
}

// edgeSettingsToState maps an API EdgeSettings response into the Terraform state model.
func edgeSettingsToState(ctx context.Context, settings *EdgeSettings, model *appEdgeSettingsResourceModel) fwdiag.Diagnostics {
	var diags fwdiag.Diagnostics

	// id mirrors app_service_id.
	model.ID = model.AppServiceID
	model.WAFMode = types.StringValue(settings.WAFMode)
	model.ConfigVersion = types.Int64Value(settings.ConfigVersion)

	// Cache rules.
	if len(settings.CacheRules) > 0 {
		ruleObjs := make([]attr.Value, len(settings.CacheRules))
		for i, cr := range settings.CacheRules {
			obj, d := types.ObjectValue(cacheRuleAttrTypes, map[string]attr.Value{
				"path_prefix": types.StringValue(cr.PathPrefix),
				"ttl_seconds": types.Int64Value(int64(cr.TTLSeconds)),
			})
			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
			ruleObjs[i] = obj
		}
		list, d := types.ListValue(cacheRuleObjectType, ruleObjs)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		model.CacheRules = list
	} else {
		emptyList, d := types.ListValue(cacheRuleObjectType, []attr.Value{})
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		model.CacheRules = emptyList
	}

	// Rate limit.
	if settings.RateLimit != nil {
		obj, d := types.ObjectValue(rateLimitAttrTypes, map[string]attr.Value{
			"requests_per_second": types.Int64Value(int64(settings.RateLimit.RequestsPerSecond)),
			"burst":               types.Int64Value(int64(settings.RateLimit.Burst)),
			"key":                 types.StringValue(settings.RateLimit.Key),
		})
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		list, d := types.ListValue(rateLimitObjectType, []attr.Value{obj})
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		model.RateLimit = list
	} else {
		emptyList, d := types.ListValue(rateLimitObjectType, []attr.Value{})
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		model.RateLimit = emptyList
	}

	return diags
}
