.PHONY: build

test:
	cd ./activities/theme-weeks && TESTING=1 go test -v -count=1
	cd ./today && TESTING=1 go test -v -count=1

build:
	GOPRIVATE=github.com/helloharbor/* sam build --parallel --cached

start_api: build
	sam local start-api --debug --log-file /tmp/out.log --env-vars ./env.json

deploy_staging: build
	@sam deploy \
	  --stack-name StagingBackendServerless \
	  --s3-bucket aws-sam-cli-managed-default-samclisourcebucket-18c48k9ep4xuk \
	  --s3-prefix StagingBackendServerless \
	  --region us-west-1 \
	  --parameter-overrides Environment=staging \
	  --capabilities CAPABILITY_IAM \
	  --confirm-changeset \
	  --profile stg

deploy_production: build
	@sam deploy \
	  --stack-name ProductionBackendServerless \
	  --s3-bucket aws-sam-cli-managed-default-samclisourcebucket-15r14v4skcd1i \
	  --s3-prefix ProductionBackendServerless \
	  --region us-west-1 \
	  --parameter-overrides Environment=production \
	  --capabilities CAPABILITY_IAM \
	  --confirm-changeset \
	  --profile prod
