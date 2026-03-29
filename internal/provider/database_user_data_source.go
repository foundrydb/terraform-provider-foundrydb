package provider

import (
	"context"
	"fmt"

	"github.com/anorph/terraform-provider-foundrydb/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure databaseUserDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &databaseUserDataSource{}

// databaseUserDataSource implements the foundrydb_database_user data source.
type databaseUserDataSource struct {
	client *client.Client
}

// databaseUserDataSourceModel holds the Terraform state for foundrydb_database_user.
type databaseUserDataSourceModel struct {
	ServiceID        types.String `tfsdk:"service_id"`
	Username         types.String `tfsdk:"username"`
	Password         types.String `tfsdk:"password"`
	Host             types.String `tfsdk:"host"`
	Port             types.Int64  `tfsdk:"port"`
	Database         types.String `tfsdk:"database"`
	ConnectionString types.String `tfsdk:"connection_string"`
}

// NewDatabaseUserDataSource returns a new databaseUserDataSource factory.
func NewDatabaseUserDataSource() datasource.DataSource {
	return &databaseUserDataSource{}
}

func (d *databaseUserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_user"
}

func (d *databaseUserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves the credentials for a database user on a FoundryDB managed service. The `password` and `connection_string` attributes are marked sensitive.",
		Attributes: map[string]schema.Attribute{
			"service_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the managed service that owns the user.",
				Required:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Database username whose credentials to retrieve.",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Database user password.",
				Computed:            true,
				Sensitive:           true,
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "Hostname or IP address used to connect to the database.",
				Computed:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "Port number the database listens on.",
				Computed:            true,
			},
			"database": schema.StringAttribute{
				MarkdownDescription: "Default database name.",
				Computed:            true,
			},
			"connection_string": schema.StringAttribute{
				MarkdownDescription: "Full connection string ready to pass to a database driver.",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (d *databaseUserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *databaseUserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state databaseUserDataSourceModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := d.client.RevealDatabaseUserPassword(
		state.ServiceID.ValueString(),
		state.Username.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error retrieving database user credentials",
			fmt.Sprintf("Could not reveal password for user %q on service %q: %s",
				state.Username.ValueString(), state.ServiceID.ValueString(), err.Error()),
		)
		return
	}

	state.Password = types.StringValue(user.Password)
	state.Host = types.StringValue(user.Host)
	state.Port = types.Int64Value(user.Port)
	state.Database = types.StringValue(user.Database)
	state.ConnectionString = types.StringValue(user.ConnectionString)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
