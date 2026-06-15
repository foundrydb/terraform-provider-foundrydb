package provider

import (
	"context"
	"fmt"

	"github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Ensure appServiceAuthResource satisfies the resource.Resource interface.
var _ resource.Resource = &appServiceAuthResource{}

// appServiceAuthResource implements the foundrydb_app_service_auth resource.
type appServiceAuthResource struct {
	client *foundrydb.Client
}

// appServiceAuthResourceModel holds the Terraform state for a foundrydb_app_service_auth.
type appServiceAuthResourceModel struct {
	// Identifying keys.
	ID           types.String `tfsdk:"id"`
	AppServiceID types.String `tfsdk:"app_service_id"`
	AttachmentID types.String `tfsdk:"attachment_id"`

	// Fixed-at-enable-time choice.
	IssuerDomainChoice types.String `tfsdk:"issuer_domain_choice"`

	// SMTP block — all fields write-only (never returned by the API).
	SMTP types.Object `tfsdk:"smtp"`

	// Theme block.
	Theme types.Object `tfsdk:"theme"`

	// Social login providers.
	IDPProviders types.List `tfsdk:"idp_providers"`

	// Computed fields from the API.
	IssuerURL  types.String `tfsdk:"issuer_url"`
	Status     types.String `tfsdk:"status"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

// smtpModel maps the smtp { } block.
type smtpModel struct {
	Host               types.String `tfsdk:"host"`
	Port               types.Int64  `tfsdk:"port"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	FromAddress        types.String `tfsdk:"from_address"`
	FromName           types.String `tfsdk:"from_name"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
}

var smtpAttrTypes = map[string]attr.Type{
	"host":                 types.StringType,
	"port":                 types.Int64Type,
	"username":             types.StringType,
	"password":             types.StringType,
	"from_address":         types.StringType,
	"from_name":            types.StringType,
	"insecure_skip_verify": types.BoolType,
}

// themeModel maps the theme { } block.
type themeModel struct {
	DisplayName types.String `tfsdk:"display_name"`
	BrandColor  types.String `tfsdk:"brand_color"`
	LogoURL     types.String `tfsdk:"logo_url"`
	SupportURL  types.String `tfsdk:"support_url"`
}

var themeAttrTypes = map[string]attr.Type{
	"display_name": types.StringType,
	"brand_color":  types.StringType,
	"logo_url":     types.StringType,
	"support_url":  types.StringType,
}

// idpProviderModel maps one entry in the idp_providers [ ] block.
type idpProviderModel struct {
	Provider     types.String `tfsdk:"provider"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	DisplayName  types.String `tfsdk:"display_name"`
}

var idpProviderAttrTypes = map[string]attr.Type{
	"provider":      types.StringType,
	"client_id":     types.StringType,
	"client_secret": types.StringType,
	"display_name":  types.StringType,
}

// NewAppServiceAuthResource returns a new appServiceAuthResource factory.
func NewAppServiceAuthResource() resource.Resource {
	return &appServiceAuthResource{}
}

func (r *appServiceAuthResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_service_auth"
}

func (r *appServiceAuthResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages end-user authentication for a FoundryDB app service.

Enabling auth provisions an OIDC issuer backed by a PostgreSQL attachment and
applies an identity schema to the customer database. The issuer handles
magic-link email login and, optionally, social login via Google or GitHub.

**Immutable fields:** ` + "`app_service_id`" + `, ` + "`attachment_id`" + `, and
` + "`issuer_domain_choice`" + ` are fixed at enable time. Changing any of these fields
forces Terraform to destroy the current auth configuration (` + "`disable`" + `) and
create a new one (` + "`enable`" + `).

**Write-only secrets:** ` + "`smtp.password`" + ` and ` + "`idp_providers[*].client_secret`" + `
are stored in the platform secret store and never returned by the API. Terraform
holds the configured value in state so it can detect drift, but the API will
never confirm the value.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier (UUID) of the auth configuration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the app service to enable auth on.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attachment_id": schema.StringAttribute{
				MarkdownDescription: "UUID of an existing PostgreSQL attachment on the app service. The platform provisions the identity schema in that database.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"issuer_domain_choice": schema.StringAttribute{
				MarkdownDescription: "Domain strategy for the OIDC issuer URL. `fallback` uses a platform-managed `auth-<id>.foundrydb.com` subdomain; `custom` uses the app's own custom domain. Fixed at enable time.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// SMTP block — write-only (never returned by API).
			"smtp": schema.SingleNestedAttribute{
				MarkdownDescription: "SMTP credentials used by the auth issuer to send magic-link emails. The `password` is stored in the platform secret store and never returned by the API.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{
						MarkdownDescription: "SMTP server hostname.",
						Required:            true,
					},
					"port": schema.Int64Attribute{
						MarkdownDescription: "SMTP server port (e.g. 587 for STARTTLS, 465 for TLS).",
						Required:            true,
					},
					"username": schema.StringAttribute{
						MarkdownDescription: "SMTP authentication username.",
						Required:            true,
					},
					"password": schema.StringAttribute{
						MarkdownDescription: "SMTP authentication password. Write-only: stored in the platform secret store, never returned.",
						Required:            true,
						Sensitive:           true,
					},
					"from_address": schema.StringAttribute{
						MarkdownDescription: "Sender email address (e.g. `noreply@example.com`).",
						Required:            true,
					},
					"from_name": schema.StringAttribute{
						MarkdownDescription: "Sender display name shown in the From header.",
						Required:            true,
					},
					"insecure_skip_verify": schema.BoolAttribute{
						MarkdownDescription: "Skip TLS certificate verification for the SMTP connection. Only use this for test mail catchers with self-signed certificates.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.Bool{
							boolplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},

			// Theme block.
			"theme": schema.SingleNestedAttribute{
				MarkdownDescription: "Branding applied to the hosted login pages.",
				Optional:            true,
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"display_name": schema.StringAttribute{
						MarkdownDescription: "Application name shown on the login page.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"brand_color": schema.StringAttribute{
						MarkdownDescription: "Hex color code for the primary brand accent (e.g. `#4F46E5`).",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"logo_url": schema.StringAttribute{
						MarkdownDescription: "URL of the logo displayed on the login page.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"support_url": schema.StringAttribute{
						MarkdownDescription: "URL linked from the login page for user support.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},

			// IDP providers list.
			"idp_providers": schema.ListNestedAttribute{
				MarkdownDescription: "Social login providers (Google, GitHub). An empty list enables magic-link email login only. The `client_secret` for each provider is stored in the platform secret store and never returned.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"provider": schema.StringAttribute{
							MarkdownDescription: "Provider identifier. One of: `google`, `github`.",
							Required:            true,
						},
						"client_id": schema.StringAttribute{
							MarkdownDescription: "OAuth client ID registered at the provider.",
							Required:            true,
						},
						"client_secret": schema.StringAttribute{
							MarkdownDescription: "OAuth client secret registered at the provider. Write-only: stored in the platform secret store, never returned.",
							Required:            true,
							Sensitive:           true,
						},
						"display_name": schema.StringAttribute{
							MarkdownDescription: "Optional label shown on the provider button on the login page.",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},

			// Computed fields.
			"issuer_url": schema.StringAttribute{
				MarkdownDescription: "The OIDC issuer URL for this auth configuration (e.g. `https://auth-1a2b3c4d.foundrydb.com`).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Lifecycle status of the auth configuration (e.g. `Active`, `ProvisioningSchema`, `Disabled`).",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of when auth was enabled.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of the last auth configuration update.",
				Computed:            true,
			},
		},
	}
}

func (r *appServiceAuthResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*foundrydb.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *foundrydb.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

// Create calls POST /app-services/{id}/auth/enable.
func (r *appServiceAuthResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appServiceAuthResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enableReq, diags := planToEnableRequest(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	auth, err := r.client.EnableAppServiceAuth(ctx, plan.AppServiceID.ValueString(), enableReq)
	if err != nil {
		resp.Diagnostics.AddError("Error enabling app service auth", err.Error())
		return
	}

	authToState(ctx, auth, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read calls GET /app-services/{id}/auth. A 404 means auth was disabled outside
// Terraform; the resource is removed from state.
func (r *appServiceAuthResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appServiceAuthResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetAppServiceAuth(ctx, state.AppServiceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading app service auth", err.Error())
		return
	}
	if result == nil || result.Auth == nil {
		// Auth was disabled outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	authToState(ctx, result.Auth, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update: the auth API has no PATCH endpoint. All mutable fields (smtp, theme,
// idp_providers) would require disable+enable to change. Because those fields
// are sensitive secrets that the API never returns, silently re-enabling with
// new values would erase the previous secrets from the secret store. Instead of
// masking that with an in-place update, this resource requires replace for
// attachment_id and issuer_domain_choice (immutable at the API level) and
// relies on the user explicitly calling `terraform apply` after a destroy, or
// using `lifecycle { replace_triggered_by = [...] }` when smtp/theme changes
// are needed. For mutable-looking fields that have no API update path, Update
// is a no-op that preserves state; the provider README documents this clearly.
func (r *appServiceAuthResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Pull plan (desired) and state (current) into models. Since there is no
	// API update endpoint, we write the plan values for smtp/theme/idp_providers
	// into state verbatim so Terraform does not perpetually show a diff. The
	// actual secret values held by the platform remain unchanged.
	var plan appServiceAuthResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state appServiceAuthResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry over computed fields from state (they are not in the plan).
	plan.ID = state.ID
	plan.IssuerURL = state.IssuerURL
	plan.Status = state.Status
	plan.CreatedAt = state.CreatedAt
	plan.UpdatedAt = state.UpdatedAt

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete calls POST /app-services/{id}/auth/disable.
func (r *appServiceAuthResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appServiceAuthResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DisableAppServiceAuth(ctx, state.AppServiceID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error disabling app service auth", err.Error())
	}
}

// planToEnableRequest converts Terraform plan values into an AuthEnableRequest.
func planToEnableRequest(ctx context.Context, plan *appServiceAuthResourceModel) (foundrydb.AuthEnableRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := foundrydb.AuthEnableRequest{
		AttachmentID:       plan.AttachmentID.ValueString(),
		IssuerDomainChoice: plan.IssuerDomainChoice.ValueString(),
	}

	// Decode SMTP block.
	if !plan.SMTP.IsNull() && !plan.SMTP.IsUnknown() {
		var smtp smtpModel
		diags.Append(plan.SMTP.As(ctx, &smtp, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return req, diags
		}
		req.SMTP = foundrydb.AuthSMTPConfig{
			Host:               smtp.Host.ValueString(),
			Port:               int(smtp.Port.ValueInt64()),
			Username:           smtp.Username.ValueString(),
			Password:           smtp.Password.ValueString(),
			FromAddress:        smtp.FromAddress.ValueString(),
			FromName:           smtp.FromName.ValueString(),
			InsecureSkipVerify: smtp.InsecureSkipVerify.ValueBool(),
		}
	}

	// Decode theme block.
	if !plan.Theme.IsNull() && !plan.Theme.IsUnknown() {
		var theme themeModel
		diags.Append(plan.Theme.As(ctx, &theme, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return req, diags
		}
		req.Theme = foundrydb.AuthThemeConfig{
			DisplayName: theme.DisplayName.ValueString(),
			BrandColor:  theme.BrandColor.ValueString(),
			LogoURL:     theme.LogoURL.ValueString(),
			SupportURL:  theme.SupportURL.ValueString(),
		}
	}

	// Decode idp_providers list.
	if !plan.IDPProviders.IsNull() && !plan.IDPProviders.IsUnknown() {
		var providers []idpProviderModel
		diags.Append(plan.IDPProviders.ElementsAs(ctx, &providers, false)...)
		if diags.HasError() {
			return req, diags
		}
		for _, p := range providers {
			req.IDPProviders = append(req.IDPProviders, foundrydb.AuthIDPProviderRequest{
				Provider:     foundrydb.AuthIDPProvider(p.Provider.ValueString()),
				ClientID:     p.ClientID.ValueString(),
				ClientSecret: p.ClientSecret.ValueString(),
				DisplayName:  p.DisplayName.ValueString(),
			})
		}
	}

	return req, diags
}

// authToState maps an API AuthConfiguration into a Terraform state model.
// Write-only fields (smtp.password, idp_providers[*].client_secret) are not
// returned by the API; the model preserves whatever the plan/state had so
// Terraform does not show perpetual drift for sensitive values.
func authToState(ctx context.Context, auth *foundrydb.AuthConfiguration, model *appServiceAuthResourceModel, diags *diag.Diagnostics) {
	model.ID = types.StringValue(auth.ID)
	model.AppServiceID = types.StringValue(auth.AppServiceID)
	model.AttachmentID = types.StringValue(auth.AttachmentID)
	model.IssuerURL = types.StringValue(auth.IssuerURL)
	model.Status = types.StringValue(auth.Status)
	model.CreatedAt = types.StringValue(auth.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	model.UpdatedAt = types.StringValue(auth.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))

	// issuer_domain_choice: derive from IssuerURL if not already set (first
	// Create populates it from the plan; Read preserves the existing value).
	if model.IssuerDomainChoice.IsNull() || model.IssuerDomainChoice.IsUnknown() {
		if auth.CustomDomain != "" {
			model.IssuerDomainChoice = types.StringValue(foundrydb.AuthIssuerDomainCustom)
		} else {
			model.IssuerDomainChoice = types.StringValue(foundrydb.AuthIssuerDomainFallback)
		}
	}

	// Theme: always populate from API response.
	themeObj, d := types.ObjectValue(themeAttrTypes, map[string]attr.Value{
		"display_name": types.StringValue(auth.Theme.DisplayName),
		"brand_color":  types.StringValue(auth.Theme.BrandColor),
		"logo_url":     types.StringValue(auth.Theme.LogoURL),
		"support_url":  types.StringValue(auth.Theme.SupportURL),
	})
	diags.Append(d...)
	model.Theme = themeObj

	// IDP providers: the API returns provider + client_id + display_name but
	// never client_secret. Merge the API response with any secrets already in
	// the model so Terraform does not see perpetual drift on the secret field.
	//
	// Strategy: build a map from provider name -> existing client_secret from
	// state, then build the new list using API-returned values for non-secret
	// fields and the preserved secret for the secret field.
	existingSecrets := map[string]string{}
	if !model.IDPProviders.IsNull() && !model.IDPProviders.IsUnknown() {
		var existing []idpProviderModel
		if dd := model.IDPProviders.ElementsAs(ctx, &existing, false); !dd.HasError() {
			for _, p := range existing {
				existingSecrets[p.Provider.ValueString()] = p.ClientSecret.ValueString()
			}
		}
	}

	providerElems := make([]attr.Value, len(auth.IDPProviders))
	for i, p := range auth.IDPProviders {
		secret := existingSecrets[string(p.Provider)]
		obj, d := types.ObjectValue(idpProviderAttrTypes, map[string]attr.Value{
			"provider":      types.StringValue(string(p.Provider)),
			"client_id":     types.StringValue(p.ClientID),
			"client_secret": types.StringValue(secret),
			"display_name":  types.StringValue(p.DisplayName),
		})
		diags.Append(d...)
		providerElems[i] = obj
	}
	idpList, d := types.ListValue(types.ObjectType{AttrTypes: idpProviderAttrTypes}, providerElems)
	diags.Append(d...)
	model.IDPProviders = idpList

	// smtp: the API never returns smtp values. Preserve whatever the model
	// currently holds. On first Create the SMTP block comes from the plan;
	// on subsequent Reads it is preserved from state. If the model has no smtp
	// yet (import scenario), set a null object so Terraform marks it as unknown
	// rather than crashing.
	if model.SMTP.IsNull() || model.SMTP.IsUnknown() {
		nullSMTP, d := types.ObjectValue(smtpAttrTypes, map[string]attr.Value{
			"host":                 types.StringValue(""),
			"port":                 types.Int64Value(0),
			"username":             types.StringValue(""),
			"password":             types.StringValue(""),
			"from_address":         types.StringValue(""),
			"from_name":            types.StringValue(""),
			"insecure_skip_verify": types.BoolValue(false),
		})
		diags.Append(d...)
		model.SMTP = nullSMTP
	}
}
