run:
	go run cmd/main.go

deploy:
	gcloud functions deploy preemptivectl --runtime go111 --entry-point=Run --service-account=preemptivectl-function@brennon-loveless.iam.gserviceaccount.com --trigger-topic=preemptivectl
