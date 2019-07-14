GOOGLE_APPLICATION_CREDENTIALS := ~/.gcp/local-development-service-account.json
FUNCTION_RUNTIME := go111
FUNCTION_ENTRY_POINT := Run
FUNCTION_SERVICE_ACCOUNT := preemptivectl-function@brennon-loveless.iam.gserviceaccount.com
FUNCTION_TRIGGER_TOPIC := preemptivectl

run:
	GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS} go run cmd/main.go

deploy:
	gcloud functions deploy preemptivectl \
		--runtime=${FUNCTION_RUNTIME} \
		--entry-point=${FUNCTION_ENTRY_POINT} \
		--service-account=${FUNCTION_SERVICE_ACCOUNT} \
		--trigger-topic=${FUNCTION_TRIGGER_TOPIC}
