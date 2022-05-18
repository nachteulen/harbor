import json
import os
import zipfile

from datetime import datetime
from io import BytesIO
from typing import Any

import boto3

from common.logger import Logger

logger = Logger.make_logger('ipaws-notifications-ingestor')
s3_client = boto3.client('s3')


def ingest(event, context):
    logger.set_trace_id(context)
    try:
        alert = event['Alert']
        if alert != '' and alert is not None:
            logger.info(f'processing alert with msg {alert}')
            filename = _work(alert)
            return {'FileName': filename}
        else:
            logger.info('no new file to process')
            return ''
    except Exception as e:
        logger.error(f'failed fetching ipaws user notification message {e}', None)
        raise


def _work(msg) -> str:
    zipped_message = _zip_message(msg)
    sent_filename = _send_zip_msg_to_raw(zipped_message)
    return sent_filename


def _zip_message(msg: Any) -> Any:
    mem_zip = BytesIO()
    with zipfile.ZipFile(mem_zip, mode='w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as zf:
        zf.writestr('beacon_dump', data=msg)

    return mem_zip


def _send_zip_msg_to_raw(zip_file: Any) -> str:
    date_time = datetime.utcnow()
    year = date_time.year
    month = date_time.month
    day = date_time.day
    file_name = date_time.isoformat()[:-7] + 'Z.zip'

    bucket = os.getenv('ETL_BUCKET')
    key = f'raw/ipaws/notifications/{year}/{month}/{day}/{file_name}'

    s3_client.put_object(
        Body=zip_file.getvalue(),
        Bucket=bucket,
        Key=key
    )

    logger.info(f'uploaded file {file_name} to s3')

    return f'{bucket}/{key}'


if __name__ == '__main__':
    ingest(None, None)
