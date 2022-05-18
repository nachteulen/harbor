import os

import boto3

from common.logger import Logger

logger = Logger.make_logger('ipaws-load-pipe-notifications')


def load_pipe(event, context):
    logger.set_trace_id(context)
    try:
        if 'CsvFile' in event:
            filepath = event['CsvFile']
            if filepath != '' and filepath is not None:
                logger.info(f'processing {filepath}')
                _work(filepath)
        else:
            logger.info('no new file to process')
    except Exception as e:
        logger.error(f'failed fetching ipaws feed: {e}', None)
        raise


def _work(filepath: str) -> None:
    s3 = boto3.resource('s3')
    bucket = os.getenv('ETL_BUCKET')
    copy_source = {
        'Bucket': bucket,
        'Key': filepath
    }
    filename = filepath.split('/')[-1]
    s3.meta.client.copy(copy_source, bucket, f'stage/ipaws/notifications/snowpipe/{filename}')


if __name__ == '__main__':
    load_pipe({'FileName': 'baikal-development'}, None)
