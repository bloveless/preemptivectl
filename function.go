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

type PubSubMessage struct {
	Data []byte `json:"data"`
}

// Exec is responsible for checking the status of instances in GCP and managing them.
// It will search for instances which are going to die in the next 35 minutes and will
// remedy this in a few steps.
// 1. Spin up a new instance by resizing the target size + 1
// 2. After that new instance is ready in GKE, cordon/drain the old node. (Approx 20 minutes left to live)
// 3. When the instance has less than 10 minutes left to live the instance will be abandoned from the instance group manager
//    this will automatically decrease the target of the instance group manager back to where it was before step 1.
func (f Function) Exec() error {
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

	fmt.Println(fmt.Sprintf("Working with instance group manager %s", instanceGroupManager.Name))

	// Now we need to get the instances within that instance group manager
	instancesResponse, err := f.computeService.InstanceGroupManagers.ListManagedInstances(f.Project, f.Zone, instanceGroupManager.Name).Context(ctx).Do()
	if err != nil {
		log.Fatal(err)
	}

	instancesChanged := 0
	for _, instance := range instancesResponse.ManagedInstances {
		instanceParts := strings.Split(instance.Instance, "/")
		instanceName := instanceParts[len(instanceParts)-1]
		instance, err := f.computeService.Instances.Get(f.Project, f.Zone, instanceName).Context(ctx).Do()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(fmt.Sprintf("Working on instance %s", instanceName))

		createdTimestamp, err := time.Parse("2006-01-02T15:04:05-07:00", instance.CreationTimestamp)
		if err != nil {
			log.Fatal(err)
		}

		age := time.Now().Sub(createdTimestamp)
		dayInMinutes := 24 * 60
		minutesUntilExpiration := float64(dayInMinutes) - age.Minutes()

		fmt.Println(fmt.Sprintf("Age (m): %f - Day (m): %d - Expiration (m): %f", age.Minutes(), dayInMinutes, minutesUntilExpiration))

		if minutesUntilExpiration < 35 {
			fmt.Println(fmt.Sprintf("Adding a new instance (%s) to the instance group manager (%s)", instanceName, instanceGroupManager.Name))
			fmt.Println(fmt.Sprintf("Resizing instance group manager (%s) target size from %d to %d", instanceGroupManager.Name, instanceGroupManager.TargetSize, instanceGroupManager.TargetSize+1))
			instancesChanged += 1
		} else if minutesUntilExpiration < 20 {
			fmt.Println(fmt.Sprintf("Time to cordon/drain this instance (%s) assigned to the instance group manager (%s)", instanceName, instanceGroupManager.Name))
			instancesChanged += 1
		} else if minutesUntilExpiration < 10 {
			fmt.Println(fmt.Sprintf("Abandoning instance (%s) from the instance group manager (%s)", instanceName, instanceGroupManager.Name))
			fmt.Println(fmt.Sprintf("Abandoning instance (%s) from the instance group manager (%s)", instanceName, instanceGroupManager.Name))
		} else {
			fmt.Println(fmt.Sprintf("Instance (%s) from the instance group manager (%s) requires no action", instanceName, instanceGroupManager.Name))
		}
	}

	if instancesChanged > 0 {
		fmt.Println(fmt.Sprintf("%d instances required some action", instancesChanged))
	} else {
		fmt.Println(fmt.Sprintf("There were no instances that required any action"))
	}

	return nil
}

// resizeManagedInstances will change the number of instances within the InstanceGroupManager.
// Allows for scaling up or down.
func (f Function) resizeManagedInstances(instanceGroupManager *compute.InstanceGroupManager, newSize int64) error {
	operation, err := f.computeService.InstanceGroupManagers.Resize(f.Project, f.Zone, instanceGroupManager.Name, newSize).Do()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(operation.MarshalJSON())
	return err
}

func (f Function) drainKubernetesNode(instanceGroupManager *compute.InstanceGroupManager, instanceName string) error {
	return nil
}

func (f Function) abandonInstance(instanceGroupManager *compute.InstanceGroupManager, instanceName string) error {
	return nil
}

// run is used by google cloud to kick off the function
func Run(ctx context.Context, m PubSubMessage) error {
	log.Println(string(m.Data))

	function := Function{
		Project:              "brennon-loveless",
		Zone:                 "us-central1-a",
		GroupManagerSelector: "demon-k8s",
	}

	return function.Exec()
}
