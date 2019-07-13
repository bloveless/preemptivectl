# Introduction
The function.go code is meant to be a Google Cloud Function running against an instance group managed by GKE. It should also work outside Google Cloud Fuctions by running `go run cmd/main.go` I don't know the schedule just yet, but my aim is to be able to run it no more than every 5 minutes.

## Brief overview
This code will look through any preemtible instances within whichever instance group is provided to the GroupManagerSelector and look for any that are about to expire (I.E. the createdTimestamp is nearly 24 hours).

If any are found that are within 45 minutes of expiring then the a new node will be added to the group for each node that will be expiring.

If any are found that are within 30 minutes of expiring the node will be cordoned/drained.

This should allow ample time for kubernetes to reschedule the nodes from the drained node to the new fresh node.

Now when the preemptible instance is terminated there will be no running servies on it and it can die in peace. The instance group should momentarily burst capacity to allow for the overlap and then clean itself up automatically to return to the requested instance size.

## Setup
Create a service account which you'll use for local development. Download the json key for the service account and put it in the file gcp-development-service-account.json in the root of this project.

You'll need to assign the service account the following roles:
- Compute Viewer

And the following permissions:
- compute.instanceGroupManagers.update
