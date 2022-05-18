.PHONY: build

build:
	sam build --parallel --cached

start_lambda: build
	sam local start-lambda --debug --log-file /tmp/out.log --env-vars ./env.json

deploy_development: build
	@sam deploy \
	  --stack-name DevelopmentHarborWorkers \
	  --s3-bucket aws-sam-cli-managed-default-samclisourcebucket-1g1n2qy1acmij \
	  --s3-prefix DevelopmentHarborWorkers \
	  --region us-west-1 \
	  --parameter-overrides Environment=development \
	  --capabilities CAPABILITY_IAM \
	  --confirm-changeset \
	  --profile dev

deploy_staging: build
	@sam deploy \
	  --stack-name StagingHarborWorkers \
	  --s3-bucket aws-sam-cli-managed-default-samclisourcebucket-18c48k9ep4xuk \
	  --s3-prefix StagingHarborWorkers \
	  --region us-west-1 \
	  --parameter-overrides Environment=staging \
	  --capabilities CAPABILITY_IAM \
	  --confirm-changeset \
	  --profile stg

deploy_production: build
	@sam deploy \
	  --stack-name ProductionHarborWorkers \
	  --s3-bucket aws-sam-cli-managed-default-samclisourcebucket-15r14v4skcd1i \
	  --s3-prefix ProductionHarborWorkers \
	  --region us-west-1 \
	  --parameter-overrides Environment=production \
	  --capabilities CAPABILITY_IAM \
	  --confirm-changeset \
	  --profile prod
