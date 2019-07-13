package main

import "preemptivectl"

func main() {
	function := preemptivectl.Function{
		Project: "brennon-loveless",
		Zone: "us-central1-a",
		GroupManagerSelector: "demon-k8s",
		AuthPath: "./gcp-development-service-account.json",
	}

	function.Exec()
}
