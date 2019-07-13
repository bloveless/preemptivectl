package preemptivectl

import (
	"context"
	"fmt"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"log"
	"strings"
)

type Function struct {
	Project string
	Zone string
	GroupManagerSelector string
	AuthPath string
}

// Exec is responsible for checking the status of instances in GCP and managing them.
// It will search for instances which are going to die in the next 30 minutes and will
// remedy this in a few steps.
// 1. Spin up a new node (expand the instance group by one node)
// 2. After that new node is ready in GKE cordon/drain the old node
// 3. After the node has been cordoned and drained for 15 or so minutes we can
//    remove the node from the instance group and shut it down.
func (f Function) Exec() {
	var computeService *compute.Service
	var err error

	ctx := context.Background()
	if f.AuthPath == "" {
		computeService, err = compute.NewService(ctx)
	} else {
		computeService, err = compute.NewService(ctx, option.WithCredentialsFile(f.AuthPath))
	}
	if err != nil {
		log.Fatal(err)
	}

	instanceGroupManagers, err := computeService.InstanceGroupManagers.List(f.Project, f.Zone).Do()
	if err != nil {
		log.Fatal(err)
	}

	var instanceGroupManager *compute.InstanceGroupManager
	for _, groupManager := range instanceGroupManagers.Items {
		if strings.Contains(groupManager.Name, f.GroupManagerSelector) {
			instanceGroupManager = groupManager
			break
		}
	}

	if instanceGroupManager == nil {
		log.Fatal("unable to find \"demon-k8s\" instance group")
	}

	fmt.Println(instanceGroupManager.Name)

	// Resizing to two for fun.
	operation, err := computeService.InstanceGroupManagers.Resize(f.Project, f.Zone, instanceGroupManager.Name, 2).Do()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(operation.MarshalJSON())
}



// run is used by google cloud to kick off the function
func run() {
	function := Function{
		Project: "brennon-loveless",
		Zone: "us-central1-a",
		GroupManagerSelector: "demon-k8s",
	}
	function.Exec()
}
