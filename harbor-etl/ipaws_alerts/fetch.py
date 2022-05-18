import os
import tempfile
import zipfile

from datetime import datetime
from io import BytesIO
from typing import Any, Dict

import boto3
import requests

from lxml import etree, objectify

from common.logger import Logger

logger = Logger.make_logger('ipaws-extract-feed')
ddb_client = boto3.client('dynamodb')
s3_client = boto3.client('s3')


def extract_feed(event, context):
    logger.set_trace_id(context)
    try:
        filename = _work()
        return {"FileName": filename}
    except Exception as e:
        logger.error(f'failed fetching ipaws feed: {e}', None)
        raise


def _work() -> str:
    return _fetch_feed()


def _fetch_feed() -> str:
    feed = requests.get(os.getenv('IPAWS_URL'))
    alerts = _clean_alerts_document(feed.text)
    unique_alerts = _get_unique_alerts(alerts)
    if len(unique_alerts) > 0:
        zipped_alerts = _zip_alerts(unique_alerts)
        sent_filename = _send_zip_to_raw(zipped_alerts)
        return sent_filename

    return ''


def _send_zip_to_raw(zip_file: Any) -> str:
    date_time = datetime.utcnow()
    year = date_time.year
    month = date_time.month
    day = date_time.day
    file_name = date_time.isoformat()[:-7]+'Z.zip'

    bucket = os.getenv('ETL_BUCKET')
    key = f'raw/ipaws/alerts/{year}/{month}/{day}/{file_name}'

    s3_client.put_object(
        Body=zip_file.getvalue(),
        Bucket=bucket,
        Key=key
    )

    logger.info(f'uploaded file {file_name} to s3')

    return f'{bucket}/{key}'


def _zip_alerts(alerts: Any) -> Any:
    tree = etree.ElementTree(alerts)
    mem_zip = BytesIO()
    with tempfile.NamedTemporaryFile() as tmp:
        try:
            tree.write(tmp.name)
            with zipfile.ZipFile(mem_zip, mode='w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as zf:
                zf.write(tmp.name)
        finally:
            tmp.close()

    return mem_zip


def _write_new_alerts(alerts: Any) -> None:
    tree = etree.ElementTree(alerts)
    tree.write('test.xml')


def _get_unique_alerts(root: Any) -> Any:
    unique_alert_arr = []

    for alert in root.findall('./alert'):
        id_node = alert.find('./identifier')
        if id_node is not None:
            id_text = str(id_node.text).strip('urn:oid:')
            if _alert_is_unique(id_text):
                _write_to_historical_record(id_text)
                unique_alert_arr.append(id_text)
                continue
        root.remove(alert)

    if len(unique_alert_arr) == 0:
        logger.info('no new alerts')
    else:
        unique_alert_str = ','.join([str(elem) for elem in unique_alert_arr])
        logger.info(f'{len(unique_alert_arr)} new alerts', {'new_alerts': unique_alert_str})

    return root


def _write_to_historical_record(alert_id: str) -> None:
    ddb_client.put_item(
        TableName=os.getenv('DYNAMO_ALERTS_TABLE'),
        Item={
            'identifier': {
                'S': alert_id
            }
        }
    )


def _clean_alerts_document(doc_str: str) -> Any:
    root = etree.fromstring(doc_str.encode('utf-8'))
    root = _strip_namespaces(root)
    root = _remove_x509_data(root)

    logger.info("removed namespaces and x509 section from feed document")

    return root


def _remove_x509_data(root: Any) -> Any:
    for alert in root.findall('./alert'):
        s = alert.find('./Signature')
        alert.remove(s)

    return root


def _strip_namespaces(root: Any) -> Any:
    for elem in root.getiterator():
        elem.tag = etree.QName(elem).localname
    objectify.deannotate(root, cleanup_namespaces=True)

    return root


def _alert_is_unique(alert_id: str) -> bool:
    resp = ddb_client.query(
        TableName=os.getenv('DYNAMO_ALERTS_TABLE'),
        KeyConditionExpression='identifier = :identifier',
        ScanIndexForward=False,
        Limit=10,
        ExpressionAttributeValues={
            ':identifier': {
                'S': alert_id
            }
        })
    return resp['Count'] == 0


if __name__ == '__main__':
    extract_feed(None, None)
