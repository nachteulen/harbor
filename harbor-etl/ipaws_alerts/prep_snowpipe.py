import csv
import os

from io import BytesIO, StringIO
from zipfile import ZipFile

from typing import Any, Dict, Optional

import boto3
import untangle

from common.logger import Logger
from headers import headers

logger = Logger.make_logger('ipaws-prep-pipe')
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
        logger.error(f'failed fetching ipaws feed: {e}', None)
        raise


def _work(filename: str) -> str:
    csv_filename = filename.split('/')[-1].replace('zip', 'csv')

    alert_zip_bytes = _fetch_archive(filename)
    unzipped_payload = _unzip_alerts(alert_zip_bytes)
    parsed_stream = _parse_payload(csv_filename, unzipped_payload)
    prefix = _write_csv_to_staging(csv_filename, parsed_stream)
    return prefix


def _write_csv_to_staging(filename: str, csv_obj: StringIO) -> str:
    date_parts = filename.split('T')[0].split('-')
    year = date_parts[0]
    month = date_parts[1]
    day = date_parts[2]
    bucket = os.getenv('ETL_BUCKET')
    key = f'stage/ipaws/alerts/{year}/{month}/{day}/{filename}'

    s3_client.put_object(
        Body=csv_obj.getvalue(),
        Bucket=bucket,
        Key=key
    )

    return key


def _fetch_archive(filename: str) -> BytesIO:
    fn_parts = filename.split('/')
    bucket = fn_parts[0]
    key = '/'.join(fn_parts[1:])
    resp = s3_client.get_object(Bucket=bucket, Key=key)
    content = resp['Body']

    return BytesIO(content.read())


def _unzip_alerts(zipped_alerts: BytesIO) -> str:
    with ZipFile(zipped_alerts) as zip_file:
        unzipped_filename = zip_file.namelist()[0]
        return zip_file.read(unzipped_filename).decode('utf-8')


def _parse_payload(filename: str, unzipped_payload: str) -> StringIO:
    xml_data = untangle.parse(unzipped_payload)
    csv_in_mem = StringIO()
    csv_writer = csv.DictWriter(csv_in_mem, delimiter='|', fieldnames=headers)
    csv_writer.writeheader()
    row = {}

    try:
        for alert in xml_data.alerts.alert:
            row['source_file'] = filename
            row['alert_date'] = filename.split('/')[-1].strip('.csv')
            row['identifier'] = _get_child_cdata(alert, 'identifier').strip('urn:oid')
            row['sender'] = _get_child_cdata(alert, 'sender', False)
            row['status'] = _get_child_cdata(alert, 'status')
            row['msg_type'] = _get_child_cdata(alert, 'msgType')
            row['source'] = _get_child_cdata(alert, 'source', False)
            row['scope'] = _get_child_cdata(alert, 'scope', False)
            row['references'] = _parse_references(alert)

            info = _get_alert_info(alert, row)
            _get_info_params(info, row)
            _get_info_area(info, row)

            csv_writer.writerow(row)

    except Exception as e:
        logger.error(e)

    return csv_in_mem


def _parse_references(alert: untangle.Element) -> str:
    refs_arr = []
    refs = _get_child_cdata(alert, 'references', False).split(' ')
    for ref in refs:
        ref_id_arr = ref.split(',')
        if len(ref_id_arr) == 3:
            ref_id = ref_id_arr[1].strip('urn:iod:')
        else:
            ref_id = ref_id_arr[0]
        refs_arr.append(ref_id)

    return ','.join(refs_arr)


def _get_alert_info(alert: untangle.Element, row: Dict[Any, Any]) -> untangle.Element:
    info = _get_child(alert, 'info')

    row['info_category'] = _get_child_cdata(info, 'category', False)
    row['info_event'] = _get_child_cdata(info, 'event', False)
    row['info_responseType'] = _get_child_cdata(info, 'responseType', False)
    row['info_urgency'] = _get_child_cdata(info, 'urgency', False)
    row['info_severity'] = _get_child_cdata(info, 'severity', False)
    row['info_certainty'] = _get_child_cdata(info, 'certainty', False)
    row['info_eventCodeSame'] = _get_event_code(info, 'SAME')
    row['info_eventCodeNWS'] = _get_event_code(info, 'NationalWeatherService')
    row['info_effective'] = _get_child_cdata(info, 'effective', False)
    row['info_onset'] = _get_child_cdata(info, 'onset', False)
    row['info_expires'] = _get_child_cdata(info, 'expires', False)
    row['info_senderName'] = _get_child_cdata(info, 'senderName', False)
    row['info_headline'] = _get_child_cdata(info, 'headline', False)
    row['info_description'] = _get_child_cdata(info, 'description', False)
    row['info_instruction'] = _get_child_cdata(info, 'instruction', False)

    return info


def _get_info_params(info: untangle.Element, row: Dict[Any, Any]) -> None:
    params = _get_child(info, 'parameter', False)

    if params is not None:
        for param in params:
            value_name = _get_child_cdata(param, 'valueName')

            if value_name == 'NWSheadline':
                row['parameter_nwsHeadline'] = _get_child_cdata(param, 'value')
            elif value_name == 'expiredReferences':
                ext_refs = []
                exp_refs = _get_child_cdata(param, 'value').split(' ')
                for exp_ref in exp_refs:
                    ref_arr = exp_ref.split(',')
                    if len(ref_arr) == 3:
                        ref = ref_arr[1].strip('urn:oid:')
                    else:
                        ref = ref_arr[0]
                    ext_refs.append(ref)
                row['parameter_expiredReferences'] = ','.join(ext_refs)


def _get_info_area(info: untangle.Element, row: Dict[Any, Any]) -> None:
    area = _get_child(info, 'area', True)

    row['area_desc'] = _get_child_cdata(area, 'areaDesc', False)
    row['area_polygon'] = _get_child_cdata(area, 'polygon', False)
    row['area_geocodesSame'] = _get_geocodes(area, 'SAME')
    row['area_geocodesUGC'] = _get_geocodes(area, 'UGC')


def _get_event_code(info: untangle.Element, tag: str) -> str:
    event_codes = _get_child(info, 'eventCode', False)

    if event_codes is not None:
        for event_code in event_codes:
            value_name = _get_child_cdata(event_code, 'valueName')

            if value_name == tag:
                return _get_child_cdata(event_code, 'value')

    return ''


def _get_geocodes(area: untangle.Element, tag: str) -> str:
    geocode_arr = []
    geocodes = _get_child(area, 'geocode', False)

    for geo in geocodes:
        value_name = _get_child_cdata(geo, 'valueName')

        if value_name == tag:
            geo_val = _get_child_cdata(geo, 'value')
            geocode_arr.append(geo_val)

    return ','.join(geocode_arr)


def _get_child(root: untangle.Element, tag: str, is_required=True) -> Optional[untangle.Element]:
    if hasattr(root, tag):
        return getattr(root, tag)
    else:
        if is_required:
            raise Exception(f'field {tag} is required')
        else:
            return None


def _get_child_cdata(root: untangle.Element, tag: str, is_required=True) -> str:
    if hasattr(root, tag):
        return getattr(root, tag).cdata
    else:
        if is_required:
            raise Exception(f'field {tag} is required')
        else:
            return ''


if __name__ == '__main__':
    prep_pipe({'FileName': 'baikal-development/raw/ipaws/2021/10/5/2021-10-05T18:42:16Z.zip'}, None)
