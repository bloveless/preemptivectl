## Setup
Create a service account which you'll use for local development. Download the json key for the service account and put it in the file cmd/gcp-development-service-account.json

You'll need to assign the service account the following roles:
- Compute Viewer

And the following permissions:
- compute.instanceGroupManagers.update
