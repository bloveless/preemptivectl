package preemptivectl

import (
	"context"
	"fmt"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"log"
	"strings"
	"time"
)

type Function struct {
	Project              string
	Zone                 string
	GroupManagerSelector string
	AuthPath             string
	computeService       *compute.Service
}

// Exec is responsible for checking the status of instances in GCP and managing them.
// It will search for instances which are going to die in the next 35 minutes and will
// remedy this in a few steps.
// 1. Spin up a new node by resizing the target size + 1
// 2. After that new node is ready in GKE, cordon/drain the old node. (Approx 20 minutes left to live)
// 3. When the node has less than 10 minutes left to live the node will be abandoned from the instance group manager
//    this will automatically decrease the target of the instance group manager back to where it was before step 1.
func (f Function) Exec() {
	var err error

	ctx := context.Background()
	if f.AuthPath == "" {
		f.computeService, err = compute.NewService(ctx)
	} else {
		f.computeService, err = compute.NewService(ctx, option.WithCredentialsFile(f.AuthPath))
	}
	if err != nil {
		log.Fatal(err)
	}

	instanceGroupManagers, err := f.computeService.InstanceGroupManagers.List(f.Project, f.Zone).Context(ctx).Do()
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

	// Now we need to get the instances within that instance group manager
	instancesResponse, err := f.computeService.InstanceGroupManagers.ListManagedInstances(f.Project, f.Zone, instanceGroupManager.Name).Context(ctx).Do()
	if err != nil {
		log.Fatal(err)
	}

	for _, instance := range instancesResponse.ManagedInstances {
		instanceParts := strings.Split(instance.Instance, "/")
		instance, err := f.computeService.Instances.Get(f.Project, f.Zone, instanceParts[len(instanceParts) - 1]).Context(ctx).Do()
		if err != nil {
			log.Fatal(err)
		}

		createdTimestamp, err := time.Parse("2006-01-02T15:04:05-07:00", instance.CreationTimestamp)
		if err != nil {
			log.Fatal(err)
		}

		age := time.Now().Sub(createdTimestamp)
		ageInMinutes := int(age.Hours() * 60 + age.Minutes())
		dayInMinutes := 24 * 60

		fmt.Println(fmt.Sprintf("Age (m): %d - Day (m): %d - Expiration (m): %d", ageInMinutes, dayInMinutes, dayInMinutes - ageInMinutes))
	}

	// Now we have our instance group manager we can adjust the target size to compensate for the expiring node.
	fmt.Println(instanceGroupManager.TargetSize)
	fmt.Println(instanceGroupManager.TargetPools)
	fmt.Println(instanceGroupManager.InstanceGroup)
	// f.resizeManagedInstances(instanceGroupManager, instanceGroupManager.TargetSize + 1)
}

// resizeManagedInstances will change the number of instances within the InstanceGroupManager.
// Allows for scaling up or down.
func (f Function) resizeManagedInstances(instanceGroupManager *compute.InstanceGroupManager, newSize int64) {
	operation, err := f.computeService.InstanceGroupManagers.Resize(f.Project, f.Zone, instanceGroupManager.Name, newSize).Do()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(operation.MarshalJSON())
}

// run is used by google cloud to kick off the function
func run() {
	function := Function{
		Project:              "brennon-loveless",
		Zone:                 "us-central1-a",
		GroupManagerSelector: "demon-k8s",
	}
	function.Exec()
}
