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

// cacheKeyModel is the Terraform representation of EdgeCacheKey.
type cacheKeyModel struct {
	VaryQueryParams types.List `tfsdk:"vary_query_params"`
	VaryHeaders     types.List `tfsdk:"vary_headers"`
	VaryCookies     types.List `tfsdk:"vary_cookies"`
}

// cacheRuleModel is the Terraform representation of one EdgeCacheRule.
type cacheRuleModel struct {
	PathPrefix                  types.String `tfsdk:"path_prefix"`
	TTLSeconds                  types.Int64  `tfsdk:"ttl_seconds"`
	StaleWhileRevalidateSeconds types.Int64  `tfsdk:"stale_while_revalidate_seconds"`
	StaleIfErrorSeconds         types.Int64  `tfsdk:"stale_if_error_seconds"`
	RequestCollapsing           types.Bool   `tfsdk:"request_collapsing"`
	CacheKey                    types.List   `tfsdk:"cache_key"`
}

// rateLimitModel is the Terraform representation of EdgeRateLimit.
type rateLimitModel struct {
	RequestsPerSecond types.Int64  `tfsdk:"requests_per_second"`
	Burst             types.Int64  `tfsdk:"burst"`
	Key               types.String `tfsdk:"key"`
}

// jwtClaimModel is the Terraform representation of EdgeJWTClaim.
type jwtClaimModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

// jwtAuthModel is the Terraform representation of EdgeJWTAuth.
type jwtAuthModel struct {
	Enabled             types.Bool   `tfsdk:"enabled"`
	Paths               types.List   `tfsdk:"paths"`
	JWKSURL             types.String `tfsdk:"jwks_url"`
	PublicKeys          types.List   `tfsdk:"public_keys"`
	Issuer              types.String `tfsdk:"issuer"`
	Audiences           types.List   `tfsdk:"audiences"`
	RequiredClaims      types.List   `tfsdk:"required_claims"`
	ForwardClaimsHeader types.String `tfsdk:"forward_claims_header"`
}

// signedURLsModel is the Terraform representation of EdgeSignedURLs.
type signedURLsModel struct {
	Enabled        types.Bool   `tfsdk:"enabled"`
	Paths          types.List   `tfsdk:"paths"`
	SecretName     types.String `tfsdk:"secret_name"`
	TTLSeconds     types.Int64  `tfsdk:"ttl_seconds"`
	SignatureParam types.String `tfsdk:"signature_param"`
	ExpiresParam   types.String `tfsdk:"expires_param"`
}

// apiKeyModel is the Terraform representation of one inbound API key.
type apiKeyModel struct {
	Name     types.String `tfsdk:"name"`
	Key      types.String `tfsdk:"key"`
	RateTier types.List   `tfsdk:"rate_tier"`
}

// apiKeyAuthModel is the Terraform representation of EdgeAPIKeyAuthRequest.
type apiKeyAuthModel struct {
	Enabled     types.Bool   `tfsdk:"enabled"`
	Paths       types.List   `tfsdk:"paths"`
	KeyLocation types.String `tfsdk:"key_location"`
	KeyName     types.String `tfsdk:"key_name"`
	Keys        types.List   `tfsdk:"keys"`
}

// wafExclusionModel is the Terraform representation of EdgeWAFExclusion.
type wafExclusionModel struct {
	RuleID types.Int64  `tfsdk:"rule_id"`
	Target types.String `tfsdk:"target"`
}

// ddosProfileModel is the Terraform representation of EdgeDDoSProfile.
type ddosProfileModel struct {
	Enabled                types.Bool  `tfsdk:"enabled"`
	PerIPRequestsPerSecond types.Int64 `tfsdk:"per_ip_requests_per_second"`
	PerIPBurst             types.Int64 `tfsdk:"per_ip_burst"`
	PerIPConnCap           types.Int64 `tfsdk:"per_ip_conn_cap"`
}

// botManagementModel is the Terraform representation of EdgeBotManagement.
type botManagementModel struct {
	Enabled            types.Bool   `tfsdk:"enabled"`
	Action             types.String `tfsdk:"action"`
	KnownBadBots       types.Bool   `tfsdk:"known_bad_bots"`
	RateBasedHeuristic types.Bool   `tfsdk:"rate_based_heuristic"`
}

// atoProtectionModel is the Terraform representation of EdgeATOProtection.
type atoProtectionModel struct {
	Enabled                    types.Bool   `tfsdk:"enabled"`
	AuthPaths                  types.List   `tfsdk:"auth_paths"`
	FailureStatusCodes         types.List   `tfsdk:"failure_status_codes"`
	PerIPThresholdPerMin       types.Int64  `tfsdk:"per_ip_threshold_per_min"`
	PerUsernameThresholdPerMin types.Int64  `tfsdk:"per_username_threshold_per_min"`
	UsernameField              types.String `tfsdk:"username_field"`
	Action                     types.String `tfsdk:"action"`
}

// appEdgeSettingsResourceModel holds the Terraform state for a foundrydb_app_edge_settings.
type appEdgeSettingsResourceModel struct {
	ID            types.String `tfsdk:"id"`
	AppServiceID  types.String `tfsdk:"app_service_id"`
	WAFMode       types.String `tfsdk:"waf_mode"`
	CacheRules    types.List   `tfsdk:"cache_rules"`
	RateLimit     types.List   `tfsdk:"rate_limit"`
	JWTAuth       types.List   `tfsdk:"jwt_auth"`
	SignedURLs    types.List   `tfsdk:"signed_urls"`
	APIKeyAuth    types.List   `tfsdk:"api_key_auth"`
	WAFParanoiaLevel  types.Int64 `tfsdk:"waf_paranoia_level"`
	WAFRuleExclusions types.List  `tfsdk:"waf_rule_exclusions"`
	DDoSProfile   types.List  `tfsdk:"ddos_profile"`
	BotManagement types.List  `tfsdk:"bot_management"`
	ATOProtection types.List  `tfsdk:"ato_protection"`
	ConfigVersion types.Int64 `tfsdk:"config_version"`
}

// Attribute type maps for the nested object/list element types.

var cacheKeyAttrTypes = map[string]attr.Type{
	"vary_query_params": types.ListType{ElemType: types.StringType},
	"vary_headers":      types.ListType{ElemType: types.StringType},
	"vary_cookies":      types.ListType{ElemType: types.StringType},
}

var cacheRuleAttrTypes = map[string]attr.Type{
	"path_prefix":                    types.StringType,
	"ttl_seconds":                    types.Int64Type,
	"stale_while_revalidate_seconds": types.Int64Type,
	"stale_if_error_seconds":         types.Int64Type,
	"request_collapsing":             types.BoolType,
	"cache_key":                      types.ListType{ElemType: types.ObjectType{AttrTypes: cacheKeyAttrTypes}},
}

var rateLimitAttrTypes = map[string]attr.Type{
	"requests_per_second": types.Int64Type,
	"burst":               types.Int64Type,
	"key":                 types.StringType,
}

var jwtClaimAttrTypes = map[string]attr.Type{
	"name":  types.StringType,
	"value": types.StringType,
}

var jwtAuthAttrTypes = map[string]attr.Type{
	"enabled":               types.BoolType,
	"paths":                 types.ListType{ElemType: types.StringType},
	"jwks_url":              types.StringType,
	"public_keys":           types.ListType{ElemType: types.StringType},
	"issuer":                types.StringType,
	"audiences":             types.ListType{ElemType: types.StringType},
	"required_claims":       types.ListType{ElemType: types.ObjectType{AttrTypes: jwtClaimAttrTypes}},
	"forward_claims_header": types.StringType,
}

var signedURLsAttrTypes = map[string]attr.Type{
	"enabled":         types.BoolType,
	"paths":           types.ListType{ElemType: types.StringType},
	"secret_name":     types.StringType,
	"ttl_seconds":     types.Int64Type,
	"signature_param": types.StringType,
	"expires_param":   types.StringType,
}

var apiKeyAttrTypes = map[string]attr.Type{
	"name":      types.StringType,
	"key":       types.StringType,
	"rate_tier": types.ListType{ElemType: types.ObjectType{AttrTypes: rateLimitAttrTypes}},
}

var apiKeyAuthAttrTypes = map[string]attr.Type{
	"enabled":      types.BoolType,
	"paths":        types.ListType{ElemType: types.StringType},
	"key_location": types.StringType,
	"key_name":     types.StringType,
	"keys":         types.ListType{ElemType: types.ObjectType{AttrTypes: apiKeyAttrTypes}},
}

var wafExclusionAttrTypes = map[string]attr.Type{
	"rule_id": types.Int64Type,
	"target":  types.StringType,
}

var ddosProfileAttrTypes = map[string]attr.Type{
	"enabled":                    types.BoolType,
	"per_ip_requests_per_second": types.Int64Type,
	"per_ip_burst":               types.Int64Type,
	"per_ip_conn_cap":            types.Int64Type,
}

var botManagementAttrTypes = map[string]attr.Type{
	"enabled":              types.BoolType,
	"action":               types.StringType,
	"known_bad_bots":       types.BoolType,
	"rate_based_heuristic": types.BoolType,
}

var atoProtectionAttrTypes = map[string]attr.Type{
	"enabled":                        types.BoolType,
	"auth_paths":                     types.ListType{ElemType: types.StringType},
	"failure_status_codes":           types.ListType{ElemType: types.Int64Type},
	"per_ip_threshold_per_min":       types.Int64Type,
	"per_username_threshold_per_min": types.Int64Type,
	"username_field":                 types.StringType,
	"action":                         types.StringType,
}

// Object types for the list-of-one (single-block) and list-of-many surfaces.
var (
	cacheKeyObjectType      = types.ObjectType{AttrTypes: cacheKeyAttrTypes}
	cacheRuleObjectType     = types.ObjectType{AttrTypes: cacheRuleAttrTypes}
	rateLimitObjectType     = types.ObjectType{AttrTypes: rateLimitAttrTypes}
	jwtClaimObjectType      = types.ObjectType{AttrTypes: jwtClaimAttrTypes}
	jwtAuthObjectType       = types.ObjectType{AttrTypes: jwtAuthAttrTypes}
	signedURLsObjectType    = types.ObjectType{AttrTypes: signedURLsAttrTypes}
	apiKeyObjectType        = types.ObjectType{AttrTypes: apiKeyAttrTypes}
	apiKeyAuthObjectType    = types.ObjectType{AttrTypes: apiKeyAuthAttrTypes}
	wafExclusionObjectType  = types.ObjectType{AttrTypes: wafExclusionAttrTypes}
	ddosProfileObjectType   = types.ObjectType{AttrTypes: ddosProfileAttrTypes}
	botManagementObjectType = types.ObjectType{AttrTypes: botManagementAttrTypes}
	atoProtectionObjectType = types.ObjectType{AttrTypes: atoProtectionAttrTypes}
)

// NewAppEdgeSettingsResource returns a new appEdgeSettingsResource factory.
func NewAppEdgeSettingsResource() resource.Resource {
	return &appEdgeSettingsResource{}
}

func (r *appEdgeSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_edge_settings"
}

func (r *appEdgeSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	stringListAttr := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{
			MarkdownDescription: desc,
			Optional:            true,
			ElementType:         types.StringType,
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the edge settings for a FoundryDB app service: cache rules (including stale-serving, request collapsing, and a narrowed cache key), rate limiting, WAF mode, access/auth (JWT, signed URLs, API keys), and security hardening (WAF paranoia level, rule exclusions, DDoS, bot management, account-takeover protection). The settings are applied to the edge fleet via a single PUT call; the resource replaces the entire settings object on every apply. Deleting this resource resets the settings to their defaults.",
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
				MarkdownDescription: "Web application firewall mode: `off` (disabled), `detect` (log without blocking), or `block` (log and reject). Defaults to `off`.",
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
						"stale_while_revalidate_seconds": schema.Int64Attribute{
							MarkdownDescription: "Window (seconds) after TTL expiry during which a stale cached response is served while a fresh copy is revalidated in the background.",
							Optional:            true,
						},
						"stale_if_error_seconds": schema.Int64Attribute{
							MarkdownDescription: "Window (seconds) after TTL expiry during which a stale cached response is served if the origin returns an error.",
							Optional:            true,
						},
						"request_collapsing": schema.BoolAttribute{
							MarkdownDescription: "When true, concurrent cache-miss requests for the same key are collapsed into a single origin fetch.",
							Optional:            true,
						},
						"cache_key": schema.ListNestedAttribute{
							MarkdownDescription: "Optional cache-key narrowing. When present, exactly one block is expected. When absent, the full request URL is the cache key.",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"vary_query_params": stringListAttr("Query parameters to include in the cache key (all others are ignored)."),
									"vary_headers":      stringListAttr("Request headers to include in the cache key."),
									"vary_cookies":      stringListAttr("Cookies to include in the cache key."),
								},
							},
						},
					},
				},
			},
			"rate_limit": schema.ListNestedAttribute{
				MarkdownDescription: "Optional rate limit configuration. When present, exactly one block is expected. When absent, rate limiting is disabled.",
				Optional:            true,
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: rateLimitSchemaAttributes(),
				},
			},
			"jwt_auth": schema.ListNestedAttribute{
				MarkdownDescription: "Optional JWT authentication. When present, exactly one block is expected. Validates inbound JWTs on the listed paths; carries no secret material.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether JWT authentication is enforced.",
							Required:            true,
						},
						"paths":    stringListAttr("Paths on which JWT authentication is enforced. Empty means all paths."),
						"jwks_url": schema.StringAttribute{MarkdownDescription: "URL of the JWKS endpoint used to fetch verification keys.", Optional: true},
						"public_keys": stringListAttr("Static PEM public keys used to verify tokens when no JWKS URL is set."),
						"issuer":      schema.StringAttribute{MarkdownDescription: "Expected `iss` claim.", Optional: true},
						"audiences":   stringListAttr("Accepted `aud` claim values."),
						"required_claims": schema.ListNestedAttribute{
							MarkdownDescription: "Claims a token must carry (name and exact value) to be accepted.",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name":  schema.StringAttribute{MarkdownDescription: "Claim name.", Required: true},
									"value": schema.StringAttribute{MarkdownDescription: "Required claim value.", Required: true},
								},
							},
						},
						"forward_claims_header": schema.StringAttribute{MarkdownDescription: "Header name under which verified claims are forwarded to the origin.", Optional: true},
					},
				},
			},
			"signed_urls": schema.ListNestedAttribute{
				MarkdownDescription: "Optional signed-URL access enforcement. When present, exactly one block is expected. `secret_name` references a stored secret by name only; the secret value never crosses the API.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{MarkdownDescription: "Whether signed-URL access is enforced.", Required: true},
						"paths":   stringListAttr("Paths on which signed-URL access is enforced."),
						"secret_name":     schema.StringAttribute{MarkdownDescription: "Name of the stored signing secret (reference only; never the value).", Optional: true},
						"ttl_seconds":     schema.Int64Attribute{MarkdownDescription: "Maximum age of a signed URL in seconds.", Optional: true},
						"signature_param": schema.StringAttribute{MarkdownDescription: "Query parameter carrying the signature. Defaults to `sig`.", Optional: true},
						"expires_param":   schema.StringAttribute{MarkdownDescription: "Query parameter carrying the expiry. Defaults to `exp`.", Optional: true},
					},
				},
			},
			"api_key_auth": schema.ListNestedAttribute{
				MarkdownDescription: "Optional API-key authentication. When present, exactly one block is expected. Each key's `key` value is write-only: it is sent to the platform (which hashes and discards it) and never returned, so it is not refreshed from the API on read.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{MarkdownDescription: "Whether API-key authentication is enforced.", Required: true},
						"paths":   stringListAttr("Paths on which API-key authentication is enforced."),
						"key_location": schema.StringAttribute{MarkdownDescription: "Where the key is read from: `header` or `query`. Defaults to `header`.", Optional: true},
						"key_name":     schema.StringAttribute{MarkdownDescription: "Header or query parameter name carrying the key. Defaults to `X-API-Key`.", Optional: true},
						"keys": schema.ListNestedAttribute{
							MarkdownDescription: "Inbound API keys accepted at the edge.",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{MarkdownDescription: "Opaque key name (returned on read).", Required: true},
									"key": schema.StringAttribute{
										MarkdownDescription: "Plaintext API key. Write-only: hashed server-side and never returned.",
										Optional:            true,
										Sensitive:           true,
									},
									"rate_tier": schema.ListNestedAttribute{
										MarkdownDescription: "Optional per-key rate limit. When present, exactly one block is expected.",
										Optional:            true,
										NestedObject: schema.NestedAttributeObject{
											Attributes: rateLimitSchemaAttributes(),
										},
									},
								},
							},
						},
					},
				},
			},
			"waf_paranoia_level": schema.Int64Attribute{
				MarkdownDescription: "WAF paranoia level 1-4 (higher is stricter, more false positives). 0 (the default) uses the platform default of PL1.",
				Optional:            true,
			},
			"waf_rule_exclusions": schema.ListNestedAttribute{
				MarkdownDescription: "WAF rule exclusions. Each entry suppresses a rule for a target; at least one of `rule_id` or `target` must be set.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"rule_id": schema.Int64Attribute{MarkdownDescription: "Numeric WAF rule id to suppress.", Optional: true},
						"target":  schema.StringAttribute{MarkdownDescription: "Request target the exclusion applies to.", Optional: true},
					},
				},
			},
			"ddos_profile": schema.ListNestedAttribute{
				MarkdownDescription: "Optional DDoS protection profile. When present, exactly one block is expected.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{MarkdownDescription: "Whether DDoS protection is enabled.", Required: true},
						"per_ip_requests_per_second": schema.Int64Attribute{MarkdownDescription: "Sustained per-IP request rate ceiling.", Optional: true},
						"per_ip_burst":               schema.Int64Attribute{MarkdownDescription: "Per-IP burst ceiling.", Optional: true},
						"per_ip_conn_cap":            schema.Int64Attribute{MarkdownDescription: "Maximum concurrent connections per IP.", Optional: true},
					},
				},
			},
			"bot_management": schema.ListNestedAttribute{
				MarkdownDescription: "Optional bot management. When present, exactly one block is expected.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{MarkdownDescription: "Whether bot management is enabled.", Required: true},
						"action":  schema.StringAttribute{MarkdownDescription: "Action on a detected bot: `log`, `block`, or `challenge`. Defaults to `log`.", Optional: true},
						"known_bad_bots":       schema.BoolAttribute{MarkdownDescription: "Match against the known-bad-bot list.", Optional: true},
						"rate_based_heuristic": schema.BoolAttribute{MarkdownDescription: "Enable the rate-based bot heuristic.", Optional: true},
					},
				},
			},
			"ato_protection": schema.ListNestedAttribute{
				MarkdownDescription: "Optional account-takeover (ATO) protection on authentication paths. When present, exactly one block is expected.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled":   schema.BoolAttribute{MarkdownDescription: "Whether ATO protection is enabled.", Required: true},
						"auth_paths": stringListAttr("Authentication paths to guard."),
						"failure_status_codes": schema.ListAttribute{
							MarkdownDescription: "Response status codes treated as auth failures. Defaults to [401, 403].",
							Optional:            true,
							ElementType:         types.Int64Type,
						},
						"per_ip_threshold_per_min":       schema.Int64Attribute{MarkdownDescription: "Failures per IP per minute before the action triggers.", Optional: true},
						"per_username_threshold_per_min": schema.Int64Attribute{MarkdownDescription: "Failures per username per minute before the action triggers.", Optional: true},
						"username_field":                 schema.StringAttribute{MarkdownDescription: "Form field or JSON key carrying the username.", Optional: true},
						"action":                         schema.StringAttribute{MarkdownDescription: "Action on threshold: `alert`, `ratelimit`, or `lock`. Defaults to `alert`.", Optional: true},
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

// rateLimitSchemaAttributes returns the shared attribute map for a rate-limit
// block (used both top-level and as a per-API-key rate tier).
func rateLimitSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
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

// stringSliceFromList decodes a types.List of strings into a Go slice.
func stringSliceFromList(ctx context.Context, l types.List, diags *fwdiag.Diagnostics) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// intSliceFromList decodes a types.List of int64 into a Go []int.
func intSliceFromList(ctx context.Context, l types.List, diags *fwdiag.Diagnostics) []int {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var vals []int64
	diags.Append(l.ElementsAs(ctx, &vals, false)...)
	out := make([]int, len(vals))
	for i, v := range vals {
		out[i] = int(v)
	}
	return out
}

// expandRateLimit decodes a single-element rate_limit list into an EdgeRateLimit.
func expandRateLimit(ctx context.Context, l types.List, diags *fwdiag.Diagnostics) *EdgeRateLimit {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var limits []rateLimitModel
	diags.Append(l.ElementsAs(ctx, &limits, false)...)
	if diags.HasError() || len(limits) == 0 {
		return nil
	}
	return &EdgeRateLimit{
		RequestsPerSecond: int(limits[0].RequestsPerSecond.ValueInt64()),
		Burst:             int(limits[0].Burst.ValueInt64()),
		Key:               limits[0].Key.ValueString(),
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
		diags.Append(model.CacheRules.ElementsAs(ctx, &rules, false)...)
		if diags.HasError() {
			return req, diags
		}
		apiRules := make([]EdgeCacheRule, len(rules))
		for i, rule := range rules {
			cr := EdgeCacheRule{
				PathPrefix:                  rule.PathPrefix.ValueString(),
				TTLSeconds:                  int(rule.TTLSeconds.ValueInt64()),
				StaleWhileRevalidateSeconds: int(rule.StaleWhileRevalidateSeconds.ValueInt64()),
				StaleIfErrorSeconds:         int(rule.StaleIfErrorSeconds.ValueInt64()),
				RequestCollapsing:           rule.RequestCollapsing.ValueBool(),
			}
			if !rule.CacheKey.IsNull() && !rule.CacheKey.IsUnknown() {
				var keys []cacheKeyModel
				diags.Append(rule.CacheKey.ElementsAs(ctx, &keys, false)...)
				if diags.HasError() {
					return req, diags
				}
				if len(keys) > 0 {
					cr.CacheKey = &EdgeCacheKey{
						VaryQueryParams: stringSliceFromList(ctx, keys[0].VaryQueryParams, &diags),
						VaryHeaders:     stringSliceFromList(ctx, keys[0].VaryHeaders, &diags),
						VaryCookies:     stringSliceFromList(ctx, keys[0].VaryCookies, &diags),
					}
				}
			}
			apiRules[i] = cr
		}
		req.CacheRules = apiRules
	}

	// Rate limit (at most one block).
	req.RateLimit = expandRateLimit(ctx, model.RateLimit, &diags)

	// JWT auth.
	if !model.JWTAuth.IsNull() && !model.JWTAuth.IsUnknown() {
		var blocks []jwtAuthModel
		diags.Append(model.JWTAuth.ElementsAs(ctx, &blocks, false)...)
		if diags.HasError() {
			return req, diags
		}
		if len(blocks) > 0 {
			b := blocks[0]
			jwt := &EdgeJWTAuth{
				Enabled:             b.Enabled.ValueBool(),
				Paths:               stringSliceFromList(ctx, b.Paths, &diags),
				JWKSURL:             b.JWKSURL.ValueString(),
				PublicKeys:          stringSliceFromList(ctx, b.PublicKeys, &diags),
				Issuer:              b.Issuer.ValueString(),
				Audiences:           stringSliceFromList(ctx, b.Audiences, &diags),
				ForwardClaimsHeader: b.ForwardClaimsHeader.ValueString(),
			}
			if !b.RequiredClaims.IsNull() && !b.RequiredClaims.IsUnknown() {
				var claims []jwtClaimModel
				diags.Append(b.RequiredClaims.ElementsAs(ctx, &claims, false)...)
				if diags.HasError() {
					return req, diags
				}
				for _, c := range claims {
					jwt.RequiredClaims = append(jwt.RequiredClaims, EdgeJWTClaim{
						Name:  c.Name.ValueString(),
						Value: c.Value.ValueString(),
					})
				}
			}
			req.JWTAuth = jwt
		}
	}

	// Signed URLs.
	if !model.SignedURLs.IsNull() && !model.SignedURLs.IsUnknown() {
		var blocks []signedURLsModel
		diags.Append(model.SignedURLs.ElementsAs(ctx, &blocks, false)...)
		if diags.HasError() {
			return req, diags
		}
		if len(blocks) > 0 {
			b := blocks[0]
			req.SignedURLs = &EdgeSignedURLs{
				Enabled:        b.Enabled.ValueBool(),
				Paths:          stringSliceFromList(ctx, b.Paths, &diags),
				SecretName:     b.SecretName.ValueString(),
				TTLSeconds:     int(b.TTLSeconds.ValueInt64()),
				SignatureParam: b.SignatureParam.ValueString(),
				ExpiresParam:   b.ExpiresParam.ValueString(),
			}
		}
	}

	// API key auth.
	if !model.APIKeyAuth.IsNull() && !model.APIKeyAuth.IsUnknown() {
		var blocks []apiKeyAuthModel
		diags.Append(model.APIKeyAuth.ElementsAs(ctx, &blocks, false)...)
		if diags.HasError() {
			return req, diags
		}
		if len(blocks) > 0 {
			b := blocks[0]
			ak := &EdgeAPIKeyAuthRequest{
				Enabled:     b.Enabled.ValueBool(),
				Paths:       stringSliceFromList(ctx, b.Paths, &diags),
				KeyLocation: b.KeyLocation.ValueString(),
				KeyName:     b.KeyName.ValueString(),
			}
			if !b.Keys.IsNull() && !b.Keys.IsUnknown() {
				var keys []apiKeyModel
				diags.Append(b.Keys.ElementsAs(ctx, &keys, false)...)
				if diags.HasError() {
					return req, diags
				}
				for _, k := range keys {
					ak.Keys = append(ak.Keys, EdgeAPIKeyRequest{
						Name:     k.Name.ValueString(),
						Key:      k.Key.ValueString(),
						RateTier: expandRateLimit(ctx, k.RateTier, &diags),
					})
				}
			}
			req.APIKeyAuth = ak
		}
	}

	// WAF paranoia level.
	if !model.WAFParanoiaLevel.IsNull() && !model.WAFParanoiaLevel.IsUnknown() {
		req.WAFParanoiaLevel = int(model.WAFParanoiaLevel.ValueInt64())
	}

	// WAF rule exclusions.
	if !model.WAFRuleExclusions.IsNull() && !model.WAFRuleExclusions.IsUnknown() {
		var excl []wafExclusionModel
		diags.Append(model.WAFRuleExclusions.ElementsAs(ctx, &excl, false)...)
		if diags.HasError() {
			return req, diags
		}
		for _, e := range excl {
			req.WAFRuleExclusions = append(req.WAFRuleExclusions, EdgeWAFExclusion{
				RuleID: int(e.RuleID.ValueInt64()),
				Target: e.Target.ValueString(),
			})
		}
	}

	// DDoS profile.
	if !model.DDoSProfile.IsNull() && !model.DDoSProfile.IsUnknown() {
		var blocks []ddosProfileModel
		diags.Append(model.DDoSProfile.ElementsAs(ctx, &blocks, false)...)
		if diags.HasError() {
			return req, diags
		}
		if len(blocks) > 0 {
			b := blocks[0]
			req.DDoSProfile = &EdgeDDoSProfile{
				Enabled:                b.Enabled.ValueBool(),
				PerIPRequestsPerSecond: int(b.PerIPRequestsPerSecond.ValueInt64()),
				PerIPBurst:             int(b.PerIPBurst.ValueInt64()),
				PerIPConnCap:           int(b.PerIPConnCap.ValueInt64()),
			}
		}
	}

	// Bot management.
	if !model.BotManagement.IsNull() && !model.BotManagement.IsUnknown() {
		var blocks []botManagementModel
		diags.Append(model.BotManagement.ElementsAs(ctx, &blocks, false)...)
		if diags.HasError() {
			return req, diags
		}
		if len(blocks) > 0 {
			b := blocks[0]
			req.BotManagement = &EdgeBotManagement{
				Enabled:            b.Enabled.ValueBool(),
				Action:             b.Action.ValueString(),
				KnownBadBots:       b.KnownBadBots.ValueBool(),
				RateBasedHeuristic: b.RateBasedHeuristic.ValueBool(),
			}
		}
	}

	// ATO protection.
	if !model.ATOProtection.IsNull() && !model.ATOProtection.IsUnknown() {
		var blocks []atoProtectionModel
		diags.Append(model.ATOProtection.ElementsAs(ctx, &blocks, false)...)
		if diags.HasError() {
			return req, diags
		}
		if len(blocks) > 0 {
			b := blocks[0]
			req.ATOProtection = &EdgeATOProtection{
				Enabled:                    b.Enabled.ValueBool(),
				AuthPaths:                  stringSliceFromList(ctx, b.AuthPaths, &diags),
				FailureStatusCodes:         intSliceFromList(ctx, b.FailureStatusCodes, &diags),
				PerIPThresholdPerMin:       int(b.PerIPThresholdPerMin.ValueInt64()),
				PerUsernameThresholdPerMin: int(b.PerUsernameThresholdPerMin.ValueInt64()),
				UsernameField:              b.UsernameField.ValueString(),
				Action:                     b.Action.ValueString(),
			}
		}
	}

	return req, diags
}

// stringListValue builds a types.List of strings, returning a null list when the
// source slice is empty so empty and unset both render the same way.
func stringListValue(ss []string, diags *fwdiag.Diagnostics) types.List {
	if len(ss) == 0 {
		return types.ListNull(types.StringType)
	}
	l, d := types.ListValueFrom(context.Background(), types.StringType, ss)
	diags.Append(d...)
	return l
}

// int64ListValue builds a types.List of int64 from a []int.
func int64ListValue(ints []int, diags *fwdiag.Diagnostics) types.List {
	if len(ints) == 0 {
		return types.ListNull(types.Int64Type)
	}
	vals := make([]int64, len(ints))
	for i, v := range ints {
		vals[i] = int64(v)
	}
	l, d := types.ListValueFrom(context.Background(), types.Int64Type, vals)
	diags.Append(d...)
	return l
}

// rateLimitToObject converts an EdgeRateLimit into a single-element list value.
func rateLimitToList(rl *EdgeRateLimit, diags *fwdiag.Diagnostics) types.List {
	if rl == nil {
		return types.ListNull(rateLimitObjectType)
	}
	obj, d := types.ObjectValue(rateLimitAttrTypes, map[string]attr.Value{
		"requests_per_second": types.Int64Value(int64(rl.RequestsPerSecond)),
		"burst":               types.Int64Value(int64(rl.Burst)),
		"key":                 types.StringValue(rl.Key),
	})
	diags.Append(d...)
	l, d := types.ListValue(rateLimitObjectType, []attr.Value{obj})
	diags.Append(d...)
	return l
}

// edgeSettingsToState maps an API EdgeSettings response into the Terraform state model.
func edgeSettingsToState(ctx context.Context, settings *EdgeSettings, model *appEdgeSettingsResourceModel) fwdiag.Diagnostics {
	var diags fwdiag.Diagnostics

	// Preserve write-only API-key plaintext from the prior model (the response
	// only returns the non-secret view).
	priorKeyPlaintext := priorAPIKeyPlaintext(ctx, model, &diags)

	// id mirrors app_service_id.
	model.ID = model.AppServiceID
	model.WAFMode = types.StringValue(settings.WAFMode)
	model.ConfigVersion = types.Int64Value(settings.ConfigVersion)

	// Cache rules.
	if len(settings.CacheRules) > 0 {
		ruleObjs := make([]attr.Value, len(settings.CacheRules))
		for i, cr := range settings.CacheRules {
			cacheKeyList := types.ListNull(cacheKeyObjectType)
			if cr.CacheKey != nil {
				ckObj, d := types.ObjectValue(cacheKeyAttrTypes, map[string]attr.Value{
					"vary_query_params": stringListValue(cr.CacheKey.VaryQueryParams, &diags),
					"vary_headers":      stringListValue(cr.CacheKey.VaryHeaders, &diags),
					"vary_cookies":      stringListValue(cr.CacheKey.VaryCookies, &diags),
				})
				diags.Append(d...)
				l, d := types.ListValue(cacheKeyObjectType, []attr.Value{ckObj})
				diags.Append(d...)
				cacheKeyList = l
			}
			obj, d := types.ObjectValue(cacheRuleAttrTypes, map[string]attr.Value{
				"path_prefix":                    types.StringValue(cr.PathPrefix),
				"ttl_seconds":                    types.Int64Value(int64(cr.TTLSeconds)),
				"stale_while_revalidate_seconds": optInt64(cr.StaleWhileRevalidateSeconds),
				"stale_if_error_seconds":         optInt64(cr.StaleIfErrorSeconds),
				"request_collapsing":             optBool(cr.RequestCollapsing),
				"cache_key":                      cacheKeyList,
			})
			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
			ruleObjs[i] = obj
		}
		list, d := types.ListValue(cacheRuleObjectType, ruleObjs)
		diags.Append(d...)
		model.CacheRules = list
	} else {
		list, d := types.ListValue(cacheRuleObjectType, []attr.Value{})
		diags.Append(d...)
		model.CacheRules = list
	}

	// Rate limit.
	model.RateLimit = rateLimitToList(settings.RateLimit, &diags)

	// JWT auth.
	model.JWTAuth = jwtAuthToList(settings.JWTAuth, &diags)

	// Signed URLs.
	model.SignedURLs = signedURLsToList(settings.SignedURLs, &diags)

	// API key auth (re-attach write-only plaintext keys by name).
	model.APIKeyAuth = apiKeyAuthToList(settings.APIKeyAuth, priorKeyPlaintext, &diags)

	// WAF paranoia level.
	model.WAFParanoiaLevel = optInt64(settings.WAFParanoiaLevel)

	// WAF rule exclusions.
	if len(settings.WAFRuleExclusions) > 0 {
		objs := make([]attr.Value, len(settings.WAFRuleExclusions))
		for i, e := range settings.WAFRuleExclusions {
			obj, d := types.ObjectValue(wafExclusionAttrTypes, map[string]attr.Value{
				"rule_id": optInt64(e.RuleID),
				"target":  optString(e.Target),
			})
			diags.Append(d...)
			objs[i] = obj
		}
		l, d := types.ListValue(wafExclusionObjectType, objs)
		diags.Append(d...)
		model.WAFRuleExclusions = l
	} else {
		model.WAFRuleExclusions = types.ListNull(wafExclusionObjectType)
	}

	// DDoS profile.
	model.DDoSProfile = ddosProfileToList(settings.DDoSProfile, &diags)

	// Bot management.
	model.BotManagement = botManagementToList(settings.BotManagement, &diags)

	// ATO protection.
	model.ATOProtection = atoProtectionToList(settings.ATOProtection, &diags)

	return diags
}

// priorAPIKeyPlaintext extracts plaintext API keys from the model's current
// api_key_auth state, keyed by key name, so a read-back can re-attach them.
func priorAPIKeyPlaintext(ctx context.Context, model *appEdgeSettingsResourceModel, diags *fwdiag.Diagnostics) map[string]string {
	out := map[string]string{}
	if model.APIKeyAuth.IsNull() || model.APIKeyAuth.IsUnknown() {
		return out
	}
	var blocks []apiKeyAuthModel
	diags.Append(model.APIKeyAuth.ElementsAs(ctx, &blocks, false)...)
	if diags.HasError() || len(blocks) == 0 {
		return out
	}
	if blocks[0].Keys.IsNull() || blocks[0].Keys.IsUnknown() {
		return out
	}
	var keys []apiKeyModel
	diags.Append(blocks[0].Keys.ElementsAs(ctx, &keys, false)...)
	for _, k := range keys {
		if !k.Key.IsNull() && !k.Key.IsUnknown() && k.Key.ValueString() != "" {
			out[k.Name.ValueString()] = k.Key.ValueString()
		}
	}
	return out
}

func jwtAuthToList(jwt *EdgeJWTAuth, diags *fwdiag.Diagnostics) types.List {
	if jwt == nil {
		return types.ListNull(jwtAuthObjectType)
	}
	claimsList := types.ListNull(jwtClaimObjectType)
	if len(jwt.RequiredClaims) > 0 {
		objs := make([]attr.Value, len(jwt.RequiredClaims))
		for i, c := range jwt.RequiredClaims {
			obj, d := types.ObjectValue(jwtClaimAttrTypes, map[string]attr.Value{
				"name":  types.StringValue(c.Name),
				"value": types.StringValue(c.Value),
			})
			diags.Append(d...)
			objs[i] = obj
		}
		l, d := types.ListValue(jwtClaimObjectType, objs)
		diags.Append(d...)
		claimsList = l
	}
	obj, d := types.ObjectValue(jwtAuthAttrTypes, map[string]attr.Value{
		"enabled":               types.BoolValue(jwt.Enabled),
		"paths":                 stringListValue(jwt.Paths, diags),
		"jwks_url":              optString(jwt.JWKSURL),
		"public_keys":           stringListValue(jwt.PublicKeys, diags),
		"issuer":                optString(jwt.Issuer),
		"audiences":             stringListValue(jwt.Audiences, diags),
		"required_claims":       claimsList,
		"forward_claims_header": optString(jwt.ForwardClaimsHeader),
	})
	diags.Append(d...)
	l, d := types.ListValue(jwtAuthObjectType, []attr.Value{obj})
	diags.Append(d...)
	return l
}

func signedURLsToList(su *EdgeSignedURLs, diags *fwdiag.Diagnostics) types.List {
	if su == nil {
		return types.ListNull(signedURLsObjectType)
	}
	obj, d := types.ObjectValue(signedURLsAttrTypes, map[string]attr.Value{
		"enabled":         types.BoolValue(su.Enabled),
		"paths":           stringListValue(su.Paths, diags),
		"secret_name":     optString(su.SecretName),
		"ttl_seconds":     optInt64(su.TTLSeconds),
		"signature_param": optString(su.SignatureParam),
		"expires_param":   optString(su.ExpiresParam),
	})
	diags.Append(d...)
	l, d := types.ListValue(signedURLsObjectType, []attr.Value{obj})
	diags.Append(d...)
	return l
}

func apiKeyAuthToList(view *EdgeAPIKeyAuthView, priorPlaintext map[string]string, diags *fwdiag.Diagnostics) types.List {
	if view == nil {
		return types.ListNull(apiKeyAuthObjectType)
	}
	keysList := types.ListNull(apiKeyObjectType)
	if len(view.Keys) > 0 {
		objs := make([]attr.Value, len(view.Keys))
		for i, k := range view.Keys {
			keyVal := types.StringNull()
			if pt, ok := priorPlaintext[k.Name]; ok {
				keyVal = types.StringValue(pt)
			}
			obj, d := types.ObjectValue(apiKeyAttrTypes, map[string]attr.Value{
				"name":      types.StringValue(k.Name),
				"key":       keyVal,
				"rate_tier": rateLimitToList(k.RateTier, diags),
			})
			diags.Append(d...)
			objs[i] = obj
		}
		l, d := types.ListValue(apiKeyObjectType, objs)
		diags.Append(d...)
		keysList = l
	}
	obj, d := types.ObjectValue(apiKeyAuthAttrTypes, map[string]attr.Value{
		"enabled":      types.BoolValue(view.Enabled),
		"paths":        stringListValue(view.Paths, diags),
		"key_location": optString(view.KeyLocation),
		"key_name":     optString(view.KeyName),
		"keys":         keysList,
	})
	diags.Append(d...)
	l, d := types.ListValue(apiKeyAuthObjectType, []attr.Value{obj})
	diags.Append(d...)
	return l
}

func ddosProfileToList(p *EdgeDDoSProfile, diags *fwdiag.Diagnostics) types.List {
	if p == nil {
		return types.ListNull(ddosProfileObjectType)
	}
	obj, d := types.ObjectValue(ddosProfileAttrTypes, map[string]attr.Value{
		"enabled":                    types.BoolValue(p.Enabled),
		"per_ip_requests_per_second": optInt64(p.PerIPRequestsPerSecond),
		"per_ip_burst":               optInt64(p.PerIPBurst),
		"per_ip_conn_cap":            optInt64(p.PerIPConnCap),
	})
	diags.Append(d...)
	l, d := types.ListValue(ddosProfileObjectType, []attr.Value{obj})
	diags.Append(d...)
	return l
}

func botManagementToList(b *EdgeBotManagement, diags *fwdiag.Diagnostics) types.List {
	if b == nil {
		return types.ListNull(botManagementObjectType)
	}
	obj, d := types.ObjectValue(botManagementAttrTypes, map[string]attr.Value{
		"enabled":              types.BoolValue(b.Enabled),
		"action":               optString(b.Action),
		"known_bad_bots":       optBool(b.KnownBadBots),
		"rate_based_heuristic": optBool(b.RateBasedHeuristic),
	})
	diags.Append(d...)
	l, d := types.ListValue(botManagementObjectType, []attr.Value{obj})
	diags.Append(d...)
	return l
}

func atoProtectionToList(a *EdgeATOProtection, diags *fwdiag.Diagnostics) types.List {
	if a == nil {
		return types.ListNull(atoProtectionObjectType)
	}
	obj, d := types.ObjectValue(atoProtectionAttrTypes, map[string]attr.Value{
		"enabled":                        types.BoolValue(a.Enabled),
		"auth_paths":                     stringListValue(a.AuthPaths, diags),
		"failure_status_codes":           int64ListValue(a.FailureStatusCodes, diags),
		"per_ip_threshold_per_min":       optInt64(a.PerIPThresholdPerMin),
		"per_username_threshold_per_min": optInt64(a.PerUsernameThresholdPerMin),
		"username_field":                 optString(a.UsernameField),
		"action":                         optString(a.Action),
	})
	diags.Append(d...)
	l, d := types.ListValue(atoProtectionObjectType, []attr.Value{obj})
	diags.Append(d...)
	return l
}

// optInt64 returns a null int64 for the zero value (the omitempty wire default),
// otherwise the value, so an unset optional attribute round-trips as null.
func optInt64(v int) types.Int64 {
	if v == 0 {
		return types.Int64Null()
	}
	return types.Int64Value(int64(v))
}

// optBool returns a null bool for false (the omitempty wire default).
func optBool(v bool) types.Bool {
	if !v {
		return types.BoolNull()
	}
	return types.BoolValue(true)
}

// optString returns a null string for the empty value (the omitempty wire default).
func optString(v string) types.String {
	if v == "" {
		return types.StringNull()
	}
	return types.StringValue(v)
}
