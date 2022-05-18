import json
import os

from common.logger import Logger

import boto3

logger = Logger.make_logger('backfill alerts')
s3 = boto3.client('s3')
lc = boto3.client('lambda')


def backfill(event, context):
    logger.set_trace_id(context)
    try:
        s3_path = event['s3_path']
        _work(s3_path)
    except Exception as e:
        logger.error(f'failed backfilling snowflake: {e}', None)
        raise


def _work(s3_path: str) -> None:
    bucket = os.getenv('ETL_BUCKET')

    paginator = s3.get_paginator('list_objects_v2')
    pages = paginator.paginate(Bucket=bucket, Prefix=s3_path)

    for page in pages:
        for obj in page['Contents']:
            key = obj['Key']
            full_path = f'{bucket}/{key}'
            lambda_payload = {'FileName': full_path}
            lc.invoke(FunctionName='IPAWSPrepSnowpipeAlerts',
                      InvocationType='Event',
                      Payload=json.dumps(lambda_payload))


if __name__ == '__main__':
    backfill({'s3_path': 'baikal-development/stage/'}, None)
