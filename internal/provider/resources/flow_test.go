package resources_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/prefecthq/terraform-provider-prefect/internal/testutils"
)

func fixtureAccFlowCreate(workspace, name, tag string) string {
	return fmt.Sprintf(`
%s

resource "prefect_flow" "flow" {
	name = "%s"
	workspace_id = prefect_workspace.test.id
	tags = ["%s"]
}
`, workspace, name, tag)
}

//nolint:paralleltest // we use the resource.ParallelTest helper instead
func TestAccResource_flow(t *testing.T) {
	resourceName := "prefect_flow.flow"
	randomName := testutils.NewRandomPrefixedString()
	workspace := testutils.NewEphemeralWorkspace()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutils.TestAccProtoV6ProviderFactories,
		PreCheck:                 func() { testutils.AccTestPreCheck(t) },
		Steps: []resource.TestStep{
			{
				// Check creation + existence of the flow resource
				Config: fixtureAccFlowCreate(workspace.Resource, randomName, "test1"),
				ConfigStateChecks: []statecheck.StateCheck{
					testutils.ExpectKnownValue(resourceName, "name", randomName),
					testutils.ExpectKnownValueList(resourceName, "tags", []string{"test1"}),
				},
			},
			{
				// Check updating the resource
				Config: fixtureAccFlowCreate(workspace.Resource, randomName, "test2"),
				ConfigStateChecks: []statecheck.StateCheck{
					testutils.ExpectKnownValue(resourceName, "name", randomName),
					testutils.ExpectKnownValueList(resourceName, "tags", []string{"test2"}),
				},
			},
			{
				// Import State checks - import by ID (default)
				ImportState:       true,
				ImportStateIdFunc: testutils.GetResourceWorkspaceImportStateID(resourceName),
				ResourceName:      resourceName,
				ImportStateVerify: true,
			},
		},
	})
}
