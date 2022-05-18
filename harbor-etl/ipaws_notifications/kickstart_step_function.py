import json
import os
import uuid

import boto3

from common.logger import Logger

logger = Logger.make_logger('ipaws-notifications-kickstart')
s3_client = boto3.client('s3')


def kickstart(event, context):
    logger.set_trace_id(context)
    try:
        body = event['Records'][0]['body']
        loaded = json.loads(body)
        msg = loaded['Message']
        logger.info(f'processing alert with msg {msg}')

        _work(msg)

    except Exception as e:
        logger.error(f'failed fetching ipaws user notification message {e}', None)
        raise


def _work(msg) -> None:
    sm_arn = os.getenv('STATE_MACHINE_ARN')
    smc = boto3.client('stepfunctions')

    response = smc.start_execution(
        stateMachineArn=sm_arn,
        name=f'IPAWS_Beacon_Notifications-{uuid.uuid4().hex}',
        input=json.dumps({'Alert': msg})
    )

    logger.info(response)


if __name__ == '__main__':
    kickstart({}, None)