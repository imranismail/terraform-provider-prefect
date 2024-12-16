package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/prefecthq/terraform-provider-prefect/internal/api"
	"github.com/prefecthq/terraform-provider-prefect/internal/provider/customtypes"
	"github.com/prefecthq/terraform-provider-prefect/internal/provider/helpers"
)

var (
	_ = resource.ResourceWithConfigure(&WebhookResource{})
	_ = resource.ResourceWithImportState(&WebhookResource{})
)

type WebhookResource struct {
	client api.PrefectClient
}

type WebhookResourceModel struct {
	ID          types.String               `tfsdk:"id"`
	Created     customtypes.TimestampValue `tfsdk:"created"`
	Updated     customtypes.TimestampValue `tfsdk:"updated"`
	Name        types.String               `tfsdk:"name"`
	Description types.String               `tfsdk:"description"`
	Enabled     types.Bool                 `tfsdk:"enabled"`
	Template    types.String               `tfsdk:"template"`
	AccountID   customtypes.UUIDValue      `tfsdk:"account_id"`
	WorkspaceID customtypes.UUIDValue      `tfsdk:"workspace_id"`
	Endpoint    types.String               `tfsdk:"endpoint"`
}

// NewWebhookResource returns a new WebhookResource.
//
//nolint:ireturn // required by Terraform API
func NewWebhookResource() resource.Resource {
	return &WebhookResource{}
}

func (r *WebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *WebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(api.PrefectClient)
	if !ok {
		resp.Diagnostics.Append(helpers.ConfigureTypeErrorDiagnostic("resource", req.ProviderData))

		return
	}

	r.client = client
}

func (r *WebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The resource `webhook` represents a Prefect Cloud Webhook. " +
			"Webhooks allow external services to trigger events in Prefect.",
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Webhook ID (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the webhook",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "Description of the webhook",
				Default:     stringdefault.StaticString(""),
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the webhook is enabled",
				Default:     booldefault.StaticBool(true),
			},
			"template": schema.StringAttribute{
				Required:    true,
				Description: "Template used by the webhook",
			},
			"created": schema.StringAttribute{
				Computed:    true,
				CustomType:  customtypes.TimestampType{},
				Description: "Timestamp of when the resource was created (RFC3339)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated": schema.StringAttribute{
				Computed:    true,
				CustomType:  customtypes.TimestampType{},
				Description: "Timestamp of when the resource was updated (RFC3339)",
			},
			"account_id": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				CustomType:  customtypes.UUIDType{},
				Description: "Account ID (UUID), defaults to the account set in the provider",
			},
			"workspace_id": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				CustomType:  customtypes.UUIDType{},
				Description: "Workspace ID (UUID), defaults to the workspace set in the provider",
			},
			"endpoint": schema.StringAttribute{
				Computed:    true,
				Description: "The fully-formed webhook endpoint, eg. https://api.prefect.cloud/SLUG",
			},
		},
	}
}

// copyWebhookResponseToModel maps an API response to a model that is saved in Terraform state.
func copyWebhookResponseToModel(webhook *api.Webhook, tfModel *WebhookResourceModel) {
	tfModel.ID = types.StringValue(webhook.ID.String())
	tfModel.Created = customtypes.NewTimestampPointerValue(&webhook.Created)
	tfModel.Updated = customtypes.NewTimestampPointerValue(&webhook.Updated)
	tfModel.Name = types.StringValue(webhook.Name)
	tfModel.Description = types.StringValue(webhook.Description)
	tfModel.Enabled = types.BoolValue(webhook.Enabled)
	tfModel.Template = types.StringValue(webhook.Template)
	tfModel.AccountID = customtypes.NewUUIDValue(webhook.AccountID)
	tfModel.WorkspaceID = customtypes.NewUUIDValue(webhook.WorkspaceID)
}

// Create creates the resource and sets the initial Terraform state.
func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WebhookResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhookClient, err := r.client.Webhooks(plan.AccountID.ValueUUID(), plan.WorkspaceID.ValueUUID())
	if err != nil {
		resp.Diagnostics.Append(helpers.CreateClientErrorDiagnostic("Webhook", err))

		return
	}

	createReq := api.WebhookCreateRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Enabled:     plan.Enabled.ValueBool(),
		Template:    plan.Template.ValueString(),
	}

	webhook, err := webhookClient.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.Append(helpers.ResourceClientErrorDiagnostic("Webhook", "create", err))

		return
	}

	// Extract the endpoint from the provider configuration.
	// https://github.com/PrefectHQ/terraform-provider-prefect/issues/333
	copyWebhookResponseToModel(webhook, &plan)
	plan.Endpoint = types.StringValue(fmt.Sprintf("https://api.prefect.cloud/%s", webhook.Slug))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *WebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.ID.IsNull() && state.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Both ID and Name are unset",
			"This is a bug in the Terraform provider. Please report it to the maintainers.",
		)

		return
	}

	client, err := r.client.Webhooks(state.AccountID.ValueUUID(), state.WorkspaceID.ValueUUID())
	if err != nil {
		resp.Diagnostics.Append(helpers.CreateClientErrorDiagnostic("Webhook", err))

		return
	}

	var webhook *api.Webhook
	if !state.ID.IsNull() {
		webhook, err = client.Get(ctx, state.ID.ValueString())
	} else {
		resp.Diagnostics.AddError(
			"ID is unset",
			"Webhook ID must be set to retrieve the resource.",
		)

		return
	}

	if err != nil {
		resp.Diagnostics.Append(helpers.ResourceClientErrorDiagnostic("Webhook", "get", err))

		return
	}

	// Extract the endpoint from the provider configuration.
	// https://github.com/PrefectHQ/terraform-provider-prefect/issues/333
	copyWebhookResponseToModel(webhook, &state)
	state.Endpoint = types.StringValue(fmt.Sprintf("https://api.prefect.cloud/%s", webhook.Slug))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *WebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WebhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state WebhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.client.Webhooks(plan.AccountID.ValueUUID(), plan.WorkspaceID.ValueUUID())
	if err != nil {
		resp.Diagnostics.Append(helpers.CreateClientErrorDiagnostic("Webhook", err))

		return
	}

	updateReq := api.WebhookUpdateRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Enabled:     plan.Enabled.ValueBool(),
		Template:    plan.Template.ValueString(),
	}

	err = client.Update(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.Append(helpers.ResourceClientErrorDiagnostic("Webhook", "update", err))

		return
	}

	webhook, err := client.Get(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.Append(helpers.ResourceClientErrorDiagnostic("Webhook", "get", err))

		return
	}

	// Extract the endpoint from the provider configuration.
	// https://github.com/PrefectHQ/terraform-provider-prefect/issues/333
	copyWebhookResponseToModel(webhook, &plan)
	plan.Endpoint = types.StringValue(fmt.Sprintf("https://api.prefect.cloud/%s", webhook.Slug))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *WebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.client.Webhooks(state.AccountID.ValueUUID(), state.WorkspaceID.ValueUUID())
	if err != nil {
		resp.Diagnostics.Append(helpers.CreateClientErrorDiagnostic("Webhook", err))

		return
	}

	err = client.Delete(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.Append(helpers.ResourceClientErrorDiagnostic("Webhook", "delete", err))

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// ImportState imports the resource into Terraform state.
func (r *WebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// we'll allow input values in the form of:
	// - "workspace_id,id"
	// - "id"
	maxInputCount := 2
	inputParts := strings.Split(req.ID, ",")

	// eg. "foo,bar,baz"
	if len(inputParts) > maxInputCount {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected a maximum of 2 import identifiers, in the form of `workspace_id,name`. Got %q", req.ID),
		)

		return
	}

	// eg. ",foo" or "foo,"
	if len(inputParts) == maxInputCount && (inputParts[0] == "" || inputParts[1] == "") {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected non-empty import identifiers, in the form of `workspace_id,name`. Got %q", req.ID),
		)

		return
	}

	if len(inputParts) == maxInputCount {
		workspaceID := inputParts[0]
		id := inputParts[1]

		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), workspaceID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	} else {
		resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	}
}
