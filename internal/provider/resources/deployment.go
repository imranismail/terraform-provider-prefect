package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/prefecthq/terraform-provider-prefect/internal/api"
	"github.com/prefecthq/terraform-provider-prefect/internal/provider/customtypes"
	"github.com/prefecthq/terraform-provider-prefect/internal/provider/helpers"
)

var (
	_ = resource.ResourceWithConfigure(&DeploymentResource{})
	_ = resource.ResourceWithImportState(&DeploymentResource{})
)

// DeploymentResource contains state for the resource.
type DeploymentResource struct {
	client api.PrefectClient
}

// DeploymentResourceModel defines the Terraform resource model.
type DeploymentResourceModel struct {
	ID      types.String               `tfsdk:"id"`
	Created customtypes.TimestampValue `tfsdk:"created"`
	Updated customtypes.TimestampValue `tfsdk:"updated"`

	AccountID   customtypes.UUIDValue `tfsdk:"account_id"`
	WorkspaceID customtypes.UUIDValue `tfsdk:"workspace_id"`

	ConcurrencyLimit       types.Int64           `tfsdk:"concurrency_limit"`
	ConcurrencyOptions     *ConcurrencyOptions   `tfsdk:"concurrency_options"`
	Description            types.String          `tfsdk:"description"`
	EnforceParameterSchema types.Bool            `tfsdk:"enforce_parameter_schema"`
	Entrypoint             types.String          `tfsdk:"entrypoint"`
	FlowID                 customtypes.UUIDValue `tfsdk:"flow_id"`
	JobVariables           jsontypes.Normalized  `tfsdk:"job_variables"`
	ManifestPath           types.String          `tfsdk:"manifest_path"`
	Name                   types.String          `tfsdk:"name"`
	ParameterOpenAPISchema jsontypes.Normalized  `tfsdk:"parameter_openapi_schema"`
	Parameters             jsontypes.Normalized  `tfsdk:"parameters"`
	Path                   types.String          `tfsdk:"path"`
	Paused                 types.Bool            `tfsdk:"paused"`
	PullSteps              []PullStepModel       `tfsdk:"pull_steps"`
	StorageDocumentID      customtypes.UUIDValue `tfsdk:"storage_document_id"`
	Tags                   types.List            `tfsdk:"tags"`
	Version                types.String          `tfsdk:"version"`
	WorkPoolName           types.String          `tfsdk:"work_pool_name"`
	WorkQueueName          types.String          `tfsdk:"work_queue_name"`
}

// ConcurrentOptions represents the concurrency options for a deployment.
type ConcurrencyOptions struct {
	// CollisionStrategy is the strategy to use when a deployment reaches its concurrency limit.
	CollisionStrategy types.String `tfsdk:"collision_strategy"`
}

// PullStepModel represents a pull step in a deployment.
type PullStepModel struct {
	// Type is the type of pull step.
	// One of:
	// - set_working_directory
	// - git_clone
	// - pull_from_azure_blob_storage
	// - pull_from_gcs
	// - pull_from_s3
	Type types.String `tfsdk:"type"`

	// Credentials is the credentials to use for the pull step.
	// Used on all PullStep types.
	Credentials types.String `tfsdk:"credentials"`

	// Requires is a list of Python package dependencies.
	Requires types.String `tfsdk:"requires"`

	//
	// Fields for set_working_directory
	//

	Directory types.String `tfsdk:"directory"`

	//
	// Fields for git_clone
	//

	// The URL of the repository to clone.
	Repository types.String `tfsdk:"repository"`

	// The branch to clone. If not provided, the default branch is used.
	Branch types.String `tfsdk:"branch"`

	// Access token for the repository.
	AccessToken types.String `tfsdk:"access_token"`

	//
	// Fields for pull_from_{cloud}
	//

	// The name of the bucket where files are stored.
	Bucket types.String `tfsdk:"bucket"`

	// The folder in the bucket where files are stored.
	Folder types.String `tfsdk:"folder"`
}

// NewDeploymentResource returns a new DeploymentResource.
//
//nolint:ireturn // required by Terraform API
func NewDeploymentResource() resource.Resource {
	return &DeploymentResource{}
}

// Metadata returns the resource type name.
func (r *DeploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

// Configure initializes runtime state for the resource.
func (r *DeploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(api.PrefectClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected api.PrefectClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Schema defines the schema for the resource.
func (r *DeploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	defaultEmptyTagList, _ := basetypes.NewListValue(types.StringType, []attr.Value{})

	resp.Schema = schema.Schema{
		// Description: "Resource representing a Prefect Workspace",
		Description: "Deployments are server-side representations of flows. " +
			"They store the crucial metadata needed for remote orchestration including when, where, and how a workflow should run. " +
			"Deployments elevate workflows from functions that you must call manually to API-managed entities that can be triggered remotely.",
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				// We cannot use a CustomType due to a conflict with PlanModifiers; see
				// https://github.com/hashicorp/terraform-plugin-framework/issues/763
				// https://github.com/hashicorp/terraform-plugin-framework/issues/754
				Description: "Workspace ID (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				CustomType:  customtypes.UUIDType{},
				Description: "Account ID (UUID), defaults to the account set in the provider",
				Optional:    true,
			},
			"workspace_id": schema.StringAttribute{
				CustomType:  customtypes.UUIDType{},
				Description: "Workspace ID (UUID) to associate deployment to",
				Optional:    true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the workspace",
				Required:    true,
			},
			"flow_id": schema.StringAttribute{
				CustomType:  customtypes.UUIDType{},
				Description: "Flow ID (UUID) to associate deployment to",
				Required:    true,
			},
			"paused": schema.BoolAttribute{
				Description: "Whether or not the deployment is paused.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"enforce_parameter_schema": schema.BoolAttribute{
				Description: "Whether or not the deployment should enforce the parameter schema.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"storage_document_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				CustomType:  customtypes.UUIDType{},
				Description: "ID of the associated storage document (UUID)",
			},
			"manifest_path": schema.StringAttribute{
				Description: "The path to the flow's manifest file, relative to the chosen storage.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"job_variables": schema.StringAttribute{
				Description: "Overrides for the flow's infrastructure configuration.",
				Optional:    true,
				Computed:    true,
				CustomType:  jsontypes.NormalizedType{},
			},
			"work_queue_name": schema.StringAttribute{
				Description: "The work queue for the deployment. If no work queue is set, work will not be scheduled.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"work_pool_name": schema.StringAttribute{
				Description: "The name of the deployment's work pool.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Description: "A description for the deployment.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Description: "The path to the working directory for the workflow, relative to remote storage or an absolute path.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"version": schema.StringAttribute{
				Description: "An optional version for the deployment.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"entrypoint": schema.StringAttribute{
				Description: "The path to the entrypoint for the workflow, relative to the path.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.ListAttribute{
				Description: "Tags associated with the deployment",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(defaultEmptyTagList),
			},
			"parameters": schema.StringAttribute{
				Description: "Parameters for flow runs scheduled by the deployment.",
				Optional:    true,
				Computed:    true,
				CustomType:  jsontypes.NormalizedType{},
			},
			"parameter_openapi_schema": schema.StringAttribute{
				Description: "The parameter schema of the flow, including defaults.",
				Optional:    true,
				Computed:    true,
				CustomType:  jsontypes.NormalizedType{},
				// OpenAPI schema is also only set on create, and
				// we do not support modifying this value. Therefore, any changes
				// to this attribute will force a replacement.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"concurrency_limit": schema.Int64Attribute{
				Description: "The deployment's concurrency limit.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"concurrency_options": schema.SingleNestedAttribute{
				Description: "Concurrency options for the deployment.",
				Optional:    true,
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"collision_strategy": schema.StringAttribute{
						Description: "Enumeration of concurrency collision strategies.",
						Required:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("ENQUEUE", "CANCEL_NEW"),
						},
					},
				},
			},
			// Pull steps are polymorphic and can have different schemas based on the pull step type.
			// In the resource schema, we only make `type` required. The other attributes are needed
			// based on the pull step type, which we'll validate in the resource layer.
			"pull_steps": schema.ListNestedAttribute{
				Description: "Pull steps to prepare flows for a deployment run.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.List{
					// Pull steps are only set on create, so any change in their value will require a resource
					// of the resource. See https://github.com/PrefectHQ/prefect/issues/11052 for more context.
					listplanmodifier.RequiresReplace(),
				},
				Default: listdefault.StaticValue(basetypes.NewListValueMust(
					types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"type":         types.StringType,
							"credentials":  types.StringType,
							"requires":     types.StringType,
							"directory":    types.StringType,
							"repository":   types.StringType,
							"branch":       types.StringType,
							"access_token": types.StringType,
							"bucket":       types.StringType,
							"folder":       types.StringType,
						},
					},
					[]attr.Value{},
				)),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Description: "The type of pull step",
							Required:    true,
							Validators: []validator.String{
								stringvalidator.OneOf(
									"set_working_directory",
									"git_clone",
									"pull_from_azure_blob_storage",
									"pull_from_gcs",
									"pull_from_s3",
								),
							},
						},
						"credentials": schema.StringAttribute{
							Description: "Credentials to use for the pull step. Refer to a {GitHub,GitLab,BitBucket} credentials block.",
							Optional:    true,
						},
						"requires": schema.StringAttribute{
							Description: "A list of Python package dependencies.",
							Optional:    true,
						},
						"directory": schema.StringAttribute{
							Description: "(For type 'set_working_directory') The directory to set as the working directory.",
							Optional:    true,
							Validators:  validatorsForConflictingAttributes(nonDirectoryAttributes),
						},
						"repository": schema.StringAttribute{
							Description: "(For type 'git_clone') The URL of the repository to clone.",
							Optional:    true,
							Validators:  validatorsForConflictingAttributes(nonGitCloneAttributes),
						},
						"branch": schema.StringAttribute{
							Description: "(For type 'git_clone') The branch to clone. If not provided, the default branch is used.",
							Optional:    true,
							Validators:  validatorsForConflictingAttributes(nonGitCloneAttributes),
						},
						"access_token": schema.StringAttribute{
							Description: "(For type 'git_clone') Access token for the repository. Refer to a credentials block for security purposes. Used in leiu of 'credentials'.",
							Optional:    true,
							Validators:  validatorsForConflictingAttributes(nonGitCloneAttributes),
						},
						"bucket": schema.StringAttribute{
							Description: "(For type 'pull_from_*') The name of the bucket where files are stored.",
							Optional:    true,
							Validators:  validatorsForConflictingAttributes(nonPullFromAttributes),
						},
						"folder": schema.StringAttribute{
							Description: "(For type 'pull_from_*') The folder in the bucket where files are stored.",
							Optional:    true,
							Validators:  validatorsForConflictingAttributes(nonPullFromAttributes),
						},
					},
				},
			},
		},
	}
}

func mapPullStepsTerraformToAPI(tfPullSteps []PullStepModel) ([]api.PullStep, diag.Diagnostics) {
	var diags diag.Diagnostics

	pullSteps := make([]api.PullStep, 0)

	for i := range tfPullSteps {
		tfPullStep := tfPullSteps[i]

		apiPullStep := api.PullStep{
			Type:        tfPullStep.Type.ValueString(),
			Credentials: tfPullStep.Credentials.ValueStringPointer(),
			Requires:    tfPullStep.Requires.ValueStringPointer(),
			Directory:   tfPullStep.Directory.ValueStringPointer(),
			Repository:  tfPullStep.Repository.ValueStringPointer(),
			Branch:      tfPullStep.Branch.ValueStringPointer(),
			AccessToken: tfPullStep.AccessToken.ValueStringPointer(),
			Bucket:      tfPullStep.Bucket.ValueStringPointer(),
			Folder:      tfPullStep.Folder.ValueStringPointer(),
		}

		pullSteps = append(pullSteps, apiPullStep)
	}

	return pullSteps, diags
}

func mapPullStepsAPIToTerraform(pullSteps []api.PullStep) ([]PullStepModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tfPullStepsModel := make([]PullStepModel, 0)

	for i := range pullSteps {
		pullStep := pullSteps[i]

		pullStepModel := PullStepModel{
			Type:        types.StringValue(pullStep.Type),
			Credentials: types.StringPointerValue(pullStep.Credentials),
			Requires:    types.StringPointerValue(pullStep.Requires),
			Directory:   types.StringPointerValue(pullStep.Directory),
			Repository:  types.StringPointerValue(pullStep.Repository),
			Branch:      types.StringPointerValue(pullStep.Branch),
			AccessToken: types.StringPointerValue(pullStep.AccessToken),
			Bucket:      types.StringPointerValue(pullStep.Bucket),
			Folder:      types.StringPointerValue(pullStep.Folder),
		}

		tfPullStepsModel = append(tfPullStepsModel, pullStepModel)
	}

	return tfPullStepsModel, diags
}

// copyDeploymentToModel copies an api.Deployment to a DeploymentResourceModel.
func copyDeploymentToModel(ctx context.Context, deployment *api.Deployment, model *DeploymentResourceModel) diag.Diagnostics {
	model.ID = types.StringValue(deployment.ID.String())
	model.Created = customtypes.NewTimestampPointerValue(deployment.Created)
	model.Updated = customtypes.NewTimestampPointerValue(deployment.Updated)

	model.Description = types.StringValue(deployment.Description)
	model.EnforceParameterSchema = types.BoolValue(deployment.EnforceParameterSchema)
	model.Entrypoint = types.StringValue(deployment.Entrypoint)
	model.FlowID = customtypes.NewUUIDValue(deployment.FlowID)
	model.ManifestPath = types.StringValue(deployment.ManifestPath)
	model.Name = types.StringValue(deployment.Name)
	model.Path = types.StringValue(deployment.Path)
	model.Paused = types.BoolValue(deployment.Paused)
	model.StorageDocumentID = customtypes.NewUUIDValue(deployment.StorageDocumentID)
	model.Version = types.StringValue(deployment.Version)
	model.WorkPoolName = types.StringValue(deployment.WorkPoolName)
	model.WorkQueueName = types.StringValue(deployment.WorkQueueName)

	tags, diags := types.ListValueFrom(ctx, types.StringType, deployment.Tags)
	if diags.HasError() {
		return diags
	}
	model.Tags = tags

	// The concurrency_limit field in the response payload is deprecated, and will always be 0
	// for compatibility. The true value has been moved under `global_concurrency_limit.limit`.
	if deployment.GlobalConcurrencyLimit != nil {
		model.ConcurrencyLimit = types.Int64Value(deployment.GlobalConcurrencyLimit.Limit)
	}

	if deployment.ConcurrencyOptions != nil {
		model.ConcurrencyOptions = &ConcurrencyOptions{
			CollisionStrategy: types.StringValue(deployment.ConcurrencyOptions.CollisionStrategy),
		}
	}

	pullSteps, diags := mapPullStepsAPIToTerraform(deployment.PullSteps)
	diags.Append(diags...)
	if diags.HasError() {
		return diags
	}

	model.PullSteps = pullSteps

	return nil
}

// Create creates the resource and sets the initial Terraform state.
func (r *DeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DeploymentResourceModel

	// Populate the model from resource configuration and emit diagnostics on error
	resp.Diagnostics.Append(req.Config.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.client.Deployments(plan.AccountID.ValueUUID(), plan.WorkspaceID.ValueUUID())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating deployment client",
			fmt.Sprintf("Could not create deployment client, unexpected error: %s. This is a bug in the provider, please report this to the maintainers.", err.Error()),
		)
	}

	var tags []string
	resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parameters, diags := helpers.SafeUnmarshal(plan.Parameters)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	jobVariables, diags := helpers.SafeUnmarshal(plan.JobVariables)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	parameterOpenAPISchema, diags := helpers.SafeUnmarshal(plan.ParameterOpenAPISchema)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	pullSteps, diags := mapPullStepsTerraformToAPI(plan.PullSteps)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createPayload := api.DeploymentCreate{
		ConcurrencyLimit:       plan.ConcurrencyLimit.ValueInt64Pointer(),
		Description:            plan.Description.ValueString(),
		EnforceParameterSchema: plan.EnforceParameterSchema.ValueBool(),
		Entrypoint:             plan.Entrypoint.ValueString(),
		FlowID:                 plan.FlowID.ValueUUID(),
		JobVariables:           jobVariables,
		ManifestPath:           plan.ManifestPath.ValueString(),
		Name:                   plan.Name.ValueString(),
		Parameters:             parameters,
		Path:                   plan.Path.ValueString(),
		Paused:                 plan.Paused.ValueBool(),
		PullSteps:              pullSteps,
		StorageDocumentID:      plan.StorageDocumentID.ValueUUIDPointer(),
		Tags:                   tags,
		Version:                plan.Version.ValueString(),
		WorkPoolName:           plan.WorkPoolName.ValueString(),
		WorkQueueName:          plan.WorkQueueName.ValueString(),
		ParameterOpenAPISchema: parameterOpenAPISchema,
	}

	if plan.ConcurrencyOptions != nil {
		createPayload.ConcurrencyOptions = &api.ConcurrencyOptions{
			CollisionStrategy: plan.ConcurrencyOptions.CollisionStrategy.ValueString(),
		}
	}

	deployment, err := client.Create(ctx, createPayload)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating deployment",
			fmt.Sprintf("Could not create deployment, unexpected error: %s", err),
		)

		return
	}

	resp.Diagnostics.Append(copyDeploymentToModel(ctx, deployment, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *DeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model DeploymentResourceModel

	// Populate the model from state and emit diagnostics on error
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.client.Deployments(model.AccountID.ValueUUID(), model.WorkspaceID.ValueUUID())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating deployment client",
			fmt.Sprintf("Could not create deployment client, unexpected error: %s. This is a bug in the provider, please report this to the maintainers.", err.Error()),
		)
	}

	// A deployment can be imported + read by either ID or Handle
	// If both are set, we prefer the ID
	var deployment *api.Deployment
	if !model.ID.IsNull() {
		var deploymentID uuid.UUID
		deploymentID, err = uuid.Parse(model.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("id"),
				"Error parsing Deployment ID",
				fmt.Sprintf("Could not parse deployment ID to UUID, unexpected error: %s", err.Error()),
			)

			return
		}

		deployment, err = client.Get(ctx, deploymentID)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error refreshing deployment state",
			fmt.Sprintf("Could not read Deployment, unexpected error: %s", err.Error()),
		)

		return
	}

	resp.Diagnostics.Append(copyDeploymentToModel(ctx, deployment, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parametersByteSlice, err := json.Marshal(deployment.Parameters)
	if err != nil {
		resp.Diagnostics.Append(helpers.SerializeDataErrorDiagnostic("parameters", "Deployment parameters", err))
	}
	model.Parameters = jsontypes.NewNormalizedValue(string(parametersByteSlice))

	jobVariablesByteSlice, err := json.Marshal(deployment.JobVariables)
	if err != nil {
		resp.Diagnostics.Append(helpers.SerializeDataErrorDiagnostic("job_variables", "Deployment job variables", err))
	}
	model.JobVariables = jsontypes.NewNormalizedValue(string(jobVariablesByteSlice))

	parameterOpenAPISchemaByteSlice, err := json.Marshal(deployment.ParameterOpenAPISchema)
	if err != nil {
		resp.Diagnostics.Append(helpers.SerializeDataErrorDiagnostic("parameter_openapi_schema", "Deployment parameter OpenAPI schema", err))
	}
	model.ParameterOpenAPISchema = jsontypes.NewNormalizedValue(string(parameterOpenAPISchemaByteSlice))

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *DeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model DeploymentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.client.Deployments(model.AccountID.ValueUUID(), model.WorkspaceID.ValueUUID())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating deployment client",
			fmt.Sprintf("Could not create deployment client, unexpected error: %s. This is a bug in the provider, please report this to the maintainers.", err.Error()),
		)

		return
	}

	deploymentID, err := uuid.Parse(model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("id"),
			"Error parsing Deployment ID",
			fmt.Sprintf("Could not parse deployment ID to UUID, unexpected error: %s", err.Error()),
		)

		return
	}

	var tags []string
	resp.Diagnostics.Append(model.Tags.ElementsAs(ctx, &tags, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var parameters map[string]interface{}
	if !model.Parameters.IsNull() {
		resp.Diagnostics.Append(model.Parameters.Unmarshal(&parameters)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var jobVariables map[string]interface{}
	if !model.JobVariables.IsNull() {
		resp.Diagnostics.Append(model.JobVariables.Unmarshal(&jobVariables)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	payload := api.DeploymentUpdate{
		ConcurrencyLimit:       model.ConcurrencyLimit.ValueInt64Pointer(),
		Description:            model.Description.ValueString(),
		EnforceParameterSchema: model.EnforceParameterSchema.ValueBool(),
		Entrypoint:             model.Entrypoint.ValueString(),
		JobVariables:           jobVariables,
		ManifestPath:           model.ManifestPath.ValueString(),
		Parameters:             parameters,
		Path:                   model.Path.ValueString(),
		Paused:                 model.Paused.ValueBool(),
		StorageDocumentID:      model.StorageDocumentID.ValueUUIDPointer(),
		Tags:                   tags,
		Version:                model.Version.ValueString(),
		WorkPoolName:           model.WorkPoolName.ValueString(),
		WorkQueueName:          model.WorkQueueName.ValueString(),
	}

	if !model.ConcurrencyOptions.CollisionStrategy.IsNull() {
		payload.ConcurrencyOptions = &api.ConcurrencyOptions{
			CollisionStrategy: model.ConcurrencyOptions.CollisionStrategy.ValueString(),
		}
	}

	err = client.Update(ctx, deploymentID, payload)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating deployment",
			fmt.Sprintf("Could not update deployment, unexpected error: %s", err),
		)

		return
	}

	deployment, err := client.Get(ctx, deploymentID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error refreshing Deployment state",
			fmt.Sprintf("Could not read Deployment, unexpected error: %s", err.Error()),
		)

		return
	}

	resp.Diagnostics.Append(copyDeploymentToModel(ctx, deployment, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parametersByteSlice, err := json.Marshal(deployment.Parameters)
	if err != nil {
		resp.Diagnostics.Append(helpers.SerializeDataErrorDiagnostic("parameters", "Deployment parameters", err))

		return
	}
	model.Parameters = jsontypes.NewNormalizedValue(string(parametersByteSlice))

	jobVariablesByteSlice, err := json.Marshal(deployment.JobVariables)
	if err != nil {
		resp.Diagnostics.Append(helpers.SerializeDataErrorDiagnostic("job_variables", "Deployment job variables", err))

		return
	}
	model.JobVariables = jsontypes.NewNormalizedValue(string(jobVariablesByteSlice))

	parameterOpenAPISchemaByteSlice, err := json.Marshal(deployment.ParameterOpenAPISchema)
	if err != nil {
		resp.Diagnostics.Append(helpers.SerializeDataErrorDiagnostic("parameter_openapi_schema", "Deployment parameter OpenAPI schema", err))

		return
	}
	model.ParameterOpenAPISchema = jsontypes.NewNormalizedValue(string(parameterOpenAPISchemaByteSlice))

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *DeploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DeploymentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.client.Deployments(state.AccountID.ValueUUID(), state.WorkspaceID.ValueUUID())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating deployment client",
			fmt.Sprintf("Could not create deployment client, unexpected error: %s. This is a bug in the provider, please report this to the maintainers.", err.Error()),
		)
	}

	deploymentID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("id"),
			"Error parsing Deployment ID",
			fmt.Sprintf("Could not parse deployment ID to UUID, unexpected error: %s", err.Error()),
		)

		return
	}

	err = client.Delete(ctx, deploymentID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting Deployment",
			fmt.Sprintf("Could not delete Deployment, unexpected error: %s", err),
		)

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// ImportState imports the resource into Terraform state.
func (r *DeploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// we'll allow input values in the form of:
	// - "id,workspace_id"
	// - "id"
	maxInputCount := 2
	inputParts := strings.Split(req.ID, ",")

	// eg. "foo,bar,baz"
	if len(inputParts) > maxInputCount {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected a maximum of 2 import identifiers, in the form of `id,workspace_id`. Got %q", req.ID),
		)

		return
	}

	// eg. ",foo" or "foo,"
	if len(inputParts) == maxInputCount && (inputParts[0] == "" || inputParts[1] == "") {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected non-empty import identifiers, in the form of `id,workspace_id`. Got %q", req.ID),
		)

		return
	}

	identifier := inputParts[0]
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), identifier)...)

	if len(inputParts) == 2 && inputParts[1] != "" {
		workspaceID, err := uuid.Parse(inputParts[1])
		if err != nil {
			resp.Diagnostics.Append(helpers.ParseUUIDErrorDiagnostic("Deployment", err))

			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), workspaceID.String())...)
	}
}

// validatorsForConflictingAttributes provides a list of string validators
// used in a ConflictsWith validator for a specific attribute.
//
// This approach is used in lieu of a ConfigValidators method because we take
// advantage of 'MatchRelative' to use the current context of the list objects
// (ListNestedAttribute).
//
// Also, expressing validators on each attribute lets us
// be more concise when defining the conflicting attributes. Defining them in
// ConfigValidators instead would be much more verbose, and disconnected from
// the source of truth.
func validatorsForConflictingAttributes(attributes []string) []validator.String {
	pathExpressions := make([]path.Expression, 0)

	for _, key := range attributes {
		pathExpressions = append(pathExpressions, path.MatchRelative().AtParent().AtName(key))
	}

	return []validator.String{
		stringvalidator.ConflictsWith(pathExpressions...),
	}
}

var (
	directoryAttributes = []string{
		"directory",
	}

	gitCloneAttributes = []string{
		"repository",
		"branch",
		"access_token",
	}

	pullFromAttributes = []string{
		"bucket",
		"folder",
	}

	nonDirectoryAttributes = append(gitCloneAttributes, pullFromAttributes...)
	nonGitCloneAttributes  = append(directoryAttributes, pullFromAttributes...)
	nonPullFromAttributes  = append(directoryAttributes, gitCloneAttributes...)
)
