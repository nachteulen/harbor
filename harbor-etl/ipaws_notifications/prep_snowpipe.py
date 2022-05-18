import csv
import os
import json

from io import BytesIO, StringIO
from typing import List
from zipfile import ZipFile

from common.logger import Logger
from headers import headers

import boto3

logger = Logger.make_logger('ipaws-notifications-prep-pipe')
s3_client = boto3.client('s3')


def prep_pipe(event, context):
    logger.set_trace_id(context)
    try:
        filename = event['FileName']
        if filename != '' and filename is not None:
            logger.info(f'processing {filename}')
            csv_prefix = _work(filename)
            return {'CsvFile': csv_prefix}
        else:
            logger.info('no new file to process')
            return ''
    except Exception as e:
        logger.error(f'failed fetching ipaws notification feed: {e}', None)
        raise


def _work(filename: str) -> List[str]:
    csv_filename = filename.split('/')[-1].replace('zip', 'csv')

    zipped_bytes = _fetch_archive(filename)
    unzipped_payload = _unzip_notifications(zipped_bytes)
    parsed_stream = _parse_payload(csv_filename, unzipped_payload)
    prefix = _write_csv_to_staging(csv_filename, parsed_stream)
    return prefix


def _write_csv_to_staging(filename: str, csv_obj: StringIO) -> str:
    date_parts = filename.split('T')[0].split('-')
    year = date_parts[0]
    month = date_parts[1]
    day = date_parts[2]
    bucket = os.getenv('ETL_BUCKET')
    key = f'stage/ipaws/notifications/{year}/{month}/{day}/{filename}'

    s3_client.put_object(
        Body=csv_obj.getvalue(),
        Bucket=bucket,
        Key=key
    )

    return key

def _parse_payload(csv_filename: str, unzipped_payload: str) -> StringIO:
    csv_in_mem = StringIO()
    csv_writer = csv.DictWriter(csv_in_mem, delimiter='|', fieldnames=headers)
    csv_writer.writeheader()
    data = json.loads(unzipped_payload)
    row = {}

    try:
        for user in data['Users']:
            row['identifier'] = data['identifier']
            if data['referenceIDs'] is not None:
                row['references'] = "".join(data['referenceIDs'])
            row['user'] = user
            row['polygon'] = data['polygon']
            row['onset_time'] = data['onsetTime']
            row['alert_date'] = csv_filename.split('T')[0]
            row['source_file'] = csv_filename

            csv_writer.writerow(row)

    except Exception as e:
        logger.error(e)

    return csv_in_mem


def _fetch_archive(filename: str) -> BytesIO:
    fn_parts = filename.split('/')
    bucket = fn_parts[0]
    key = '/'.join(fn_parts[1:])
    resp = s3_client.get_object(Bucket=bucket, Key=key)
    content = resp['Body']

    return BytesIO(content.read())


def _unzip_notifications(zipped_notifications: BytesIO) -> str:
    with ZipFile(zipped_notifications) as zip_file:
        unzipped_filename = zip_file.namelist()[0]
        return zip_file.read(unzipped_filename).decode('utf-8')


if __name__ == '__main__':
    prep_pipe({'FileName': 'development-baikal/raw/ipaws/notifications/2021/10/25/2021-10-25T00:00:51Z.zip'}, None)