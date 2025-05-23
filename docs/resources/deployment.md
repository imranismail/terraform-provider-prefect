---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "prefect_deployment Resource - prefect"
subcategory: ""
description: |-
  Deployments are server-side representations of flows. They store the crucial metadata needed for remote orchestration including when, where, and how a workflow should run. Deployments elevate workflows from functions that you must call manually to API-managed entities that can be triggered remotely. For more information, see deploy overview https://docs.prefect.io/v3/deploy/index.
  This feature is available in the following product plan(s) https://www.prefect.io/pricing: Prefect OSS, Prefect Cloud (Free), Prefect Cloud (Pro), Prefect Cloud (Enterprise).
---

# prefect_deployment (Resource)


Deployments are server-side representations of flows. They store the crucial metadata needed for remote orchestration including when, where, and how a workflow should run. Deployments elevate workflows from functions that you must call manually to API-managed entities that can be triggered remotely. For more information, see [deploy overview](https://docs.prefect.io/v3/deploy/index).

This feature is available in the following [product plan(s)](https://www.prefect.io/pricing): Prefect OSS, Prefect Cloud (Free), Prefect Cloud (Pro), Prefect Cloud (Enterprise).


## Example Usage

```terraform
resource "prefect_workspace" "workspace" {
  handle = "my-workspace"
  name   = "my-workspace"
}

resource "prefect_block" "demo_github_repository" {
  name      = "demo-github-repository"
  type_slug = "github-repository"

  data = jsonencode({
    "repository_url" : "https://github.com/foo/bar",
    "reference" : "main"
  })

  workspace_id = prefect_workspace.workspace.id
}

resource "prefect_flow" "flow" {
  name         = "my-flow"
  workspace_id = prefect_workspace.workspace.id
  tags         = ["tf-test"]
}

resource "prefect_deployment" "deployment" {
  name                     = "my-deployment"
  description              = "string"
  workspace_id             = prefect_workspace.workspace.id
  flow_id                  = prefect_flow.flow.id
  entrypoint               = "hello_world.py:hello_world"
  tags                     = ["test"]
  enforce_parameter_schema = false
  job_variables = jsonencode({
    "env" : { "some-key" : "some-value" }
  })
  manifest_path = "./bar/foo"
  parameters = jsonencode({
    "some-parameter" : "some-value",
    "some-parameter2" : "some-value2"
  })
  parameter_openapi_schema = jsonencode({
    "type" : "object",
    "properties" : {
      "some-parameter" : { "type" : "string" }
      "some-parameter2" : { "type" : "string" }
    }
  })
  path   = "./foo/bar"
  paused = false
  pull_steps = [
    {
      type      = "set_working_directory",
      directory = "/some/directory",
    },
    {
      type               = "git_clone"
      repository         = "https://github.com/some/repo"
      branch             = "main"
      include_submodules = true

      # For private repositories, choose from one of the following options:
      #
      # Option 1: using an access token by passing it as plaintext
      access_token = "123abc"
      # Option 2: using an access token by referencing a Secret block
      access_token = "{{ prefect.blocks.secret.github-token }}"
      # Option 3: using a Credentials block
      credentials = "{{ prefect.blocks.github-credentials.private-repo-creds }}"
    },
    {
      type     = "pull_from_s3",
      requires = "prefect-aws>=0.3.4"
      bucket   = "some-bucket",
      folder   = "some-folder",
    }
  ]
  storage_document_id = prefect_block.test_gh_repository.id
  version             = "v1.1.1"
  work_pool_name      = "some-testing-pool"
  work_queue_name     = "default"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `flow_id` (String) Flow ID (UUID) to associate deployment to
- `name` (String) Name of the workspace

### Optional

- `account_id` (String) Account ID (UUID), defaults to the account set in the provider
- `concurrency_limit` (Number) The deployment's concurrency limit.
- `concurrency_options` (Attributes) Concurrency options for the deployment. (see [below for nested schema](#nestedatt--concurrency_options))
- `description` (String) A description for the deployment.
- `enforce_parameter_schema` (Boolean) Whether or not the deployment should enforce the parameter schema.
- `entrypoint` (String) The path to the entrypoint for the workflow, relative to the path.
- `job_variables` (String) Overrides for the flow's infrastructure configuration.
- `manifest_path` (String) The path to the flow's manifest file, relative to the chosen storage.
- `parameter_openapi_schema` (String) The parameter schema of the flow, including defaults.
- `parameters` (String) Parameters for flow runs scheduled by the deployment.
- `path` (String) The path to the working directory for the workflow, relative to remote storage or an absolute path.
- `paused` (Boolean) Whether or not the deployment is paused.
- `pull_steps` (Attributes List) Pull steps to prepare flows for a deployment run. (see [below for nested schema](#nestedatt--pull_steps))
- `storage_document_id` (String) ID of the associated storage document (UUID)
- `tags` (List of String) Tags associated with the deployment
- `version` (String) An optional version for the deployment.
- `work_pool_name` (String) The name of the deployment's work pool.
- `work_queue_name` (String) The work queue for the deployment. If no work queue is set, work will not be scheduled.
- `workspace_id` (String) Workspace ID (UUID) to associate deployment to

### Read-Only

- `created` (String) Timestamp of when the resource was created (RFC3339)
- `id` (String) Workspace ID (UUID)
- `updated` (String) Timestamp of when the resource was updated (RFC3339)

<a id="nestedatt--concurrency_options"></a>
### Nested Schema for `concurrency_options`

Required:

- `collision_strategy` (String) Enumeration of concurrency collision strategies.


<a id="nestedatt--pull_steps"></a>
### Nested Schema for `pull_steps`

Required:

- `type` (String) The type of pull step

Optional:

- `access_token` (String) (For type 'git_clone') Access token for the repository. Refer to a credentials block for security purposes. Used in leiu of 'credentials'.
- `branch` (String) (For type 'git_clone') The branch to clone. If not provided, the default branch is used.
- `bucket` (String) (For type 'pull_from_*') The name of the bucket where files are stored.
- `credentials` (String) Credentials to use for the pull step. Refer to a {GitHub,GitLab,BitBucket} credentials block.
- `directory` (String) (For type 'set_working_directory') The directory to set as the working directory.
- `folder` (String) (For type 'pull_from_*') The folder in the bucket where files are stored.
- `include_submodules` (Boolean) (For type 'git_clone') Whether to include submodules when cloning the repository.
- `repository` (String) (For type 'git_clone') The URL of the repository to clone.
- `requires` (String) A list of Python package dependencies.

## Import

Import is supported using the following syntax:

```shell
# Prefect Deployments can be imported via deployment_id
terraform import prefect_deployment.example 00000000-0000-0000-0000-000000000000

# or from a different workspace via deployment_id,workspace_id
terraform import prefect_deployment.example 00000000-0000-0000-0000-000000000000,00000000-0000-0000-0000-000000000000
```

## Deployment actions

The deployment resource does not provide any direct equivalent to the
[`build`](https://docs.prefect.io/v3/deploy/infrastructure-concepts/prefect-yaml#the-build-action)
and [`push`](https://docs.prefect.io/v3/deploy/infrastructure-concepts/prefect-yaml#the-push-action)
actions available in the `prefect.yaml`
approach used with the Prefect CLI.

However, you can specify an image in the `job_variables` field:

```terraform
resource "prefect_deployment" "deployment" {
  name                     = "my-deployment"
  description              = "my description"
  flow_id                  = prefect_flow.flow.id

  job_variables = jsonencode({
    "image" : "example.registry.com/example-repo/example-image:v1" }
  })
}
```

This setting controls the image used in the Kubernetes Job that executes the flow.

Additionally, a provider such as [kreuzwerker/docker](https://registry.terraform.io/providers/kreuzwerker/docker/latest/docs)
may be useful if you still need to build and push images from Terraform. Otherwise,
we recommend using another mechanism to build and push images and then refer to
them by name as shown in the example above. Notably, Hashicorp also makes
Packer, which can [build Docker images](https://developer.hashicorp.com/packer/tutorials/docker-get-started/docker-get-started-build-image).
