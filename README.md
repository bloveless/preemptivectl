# Introduction
This is meant to be a Google Cloud Function but should also work wherever you can build/run the executable. It is developed localy by running `go run cmd/main.go` but the function.go file is meant to be able to be run by a Google Cloud Function periodically. I don't know the schedule just yet, but my aim is to be able to run it no more than every 10 minutes.

## Setup
Create a service account which you'll use for local development. Download the json key for the service account and put it in the file cmd/gcp-development-service-account.json

You'll need to assign the service account the following roles:
- Compute Viewer

And the following permissions:
- compute.instanceGroupManagers.update
