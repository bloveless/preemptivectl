package preemptivectl

import (
	"context"
	"fmt"
	"google.golang.org/api/compute/v1"
	"log"
	"strings"
	"time"
)

type Marshaller interface {
	MarshalJSON() ([]byte, error)
}

type Function struct {
	Project              string
	Zone                 string
	GroupManagerSelector string
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
	f.computeService, err = compute.NewService(ctx)
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
		previousStatus := ""
		instance, err := f.computeService.Instances.Get(f.Project, f.Zone, instanceName).Context(ctx).Do()
		if err != nil {
			log.Fatal(err)
		}

		// Find the previous status of the instance
		for _, metadata := range instance.Metadata.Items {
			if metadata.Key == "preemptivectl" {
				previousStatus = *metadata.Value
			}
		}

		f.setInstanceMetadata(instance, "preemptivectl", "")

		fmt.Println(fmt.Sprintf("Working on instance %s", instanceName))

		createdTimestamp, err := time.Parse("2006-01-02T15:04:05-07:00", instance.CreationTimestamp)
		if err != nil {
			log.Fatal(err)
		}

		age := time.Now().Sub(createdTimestamp)
		dayInMinutes := 24 * 60
		minutesUntilExpiration := float64(dayInMinutes) - age.Minutes()

		fmt.Println(fmt.Sprintf("Age (m): %f - Day (m): %d - Expiration (m): %f", age.Minutes(), dayInMinutes, minutesUntilExpiration))

		if previousStatus == "" && minutesUntilExpiration < 35 {
			fmt.Println(fmt.Sprintf("Adding a new instance (%s) to the instance group manager (%s)", instance.Name, instanceGroupManager.Name))
			fmt.Println(fmt.Sprintf("Resizing instance group manager (%s) target size from %d to %d", instanceGroupManager.Name, instanceGroupManager.TargetSize, instanceGroupManager.TargetSize+1))

			f.setInstanceMetadata(instance, "preemptivectl", "initiated-group-manager-resize")

			instancesChanged += 1
		} else if previousStatus == "initiated-group-manager-resize" && minutesUntilExpiration < 20 {
			fmt.Println(fmt.Sprintf("Time to cordon/drain this instance (%s) assigned to the instance group manager (%s)", instance.Name, instanceGroupManager.Name))

			f.setInstanceMetadata(instance, "preemptivectl", "instance-drained")

			instancesChanged += 1
		} else if previousStatus == "instance-drained" && minutesUntilExpiration < 10 {
			fmt.Println(fmt.Sprintf("Abandoning instance (%s) from the instance group manager (%s)", instance.Name, instanceGroupManager.Name))
			fmt.Println(fmt.Sprintf("Abandoning instance (%s) from the instance group manager (%s)", instance.Name, instanceGroupManager.Name))

			f.setInstanceMetadata(instance, "preemptivectl", "complete")
		} else {
			fmt.Println(fmt.Sprintf("Instance (%s) from the instance group manager (%s) requires no action", instance.Name, instanceGroupManager.Name))
		}
	}

	if instancesChanged > 0 {
		fmt.Println(fmt.Sprintf("%d instances required some action", instancesChanged))
	} else {
		fmt.Println(fmt.Sprintf("There were no instances that required any action"))
	}

	return nil
}

// setInstanceMetadata will add a new instance metadata key to the specified instance
func (f Function) setInstanceMetadata(instance *compute.Instance, key, value string) {
	foundKey := false
	for _, metadataItem := range instance.Metadata.Items {
		if metadataItem.Key == key {
			metadataItem.Value = &value
			foundKey = true
		}
	}

	if !foundKey {
		instance.Metadata.Items = append(instance.Metadata.Items, &compute.MetadataItems{
			Key: key,
			Value: &value,
		})
	}

	operation, err := f.computeService.Instances.SetMetadata(f.Project, f.Zone, instance.Name, instance.Metadata).Do()
	if err != nil {
		log.Fatal(err)
	}

	f.printJson(operation)
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

func (f Function) printJson(v Marshaller) {
	json, err := v.MarshalJSON()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(json))
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
