from typing import List

import psycopg2

from typing import Dict, Any, Optional


class PostgresClient:
    def __init__(self, host: str, user: str, password: str, db_name: str, port='5432'):
        self._host: str = host
        self._user: str = user
        self._password: str = password
        self._db_name: str = db_name
        self._port: str = port
        self._conn: None

        self._make_connection()

    def _make_connection(self):

        print(f'{self._port}, {self._host}, {self._user}, {self._password}, {self._db_name}')

        self._conn = psycopg2.connect(
            user=self._user,
            password=self._password,
            host=self._host,
            dbname=self._db_name,
            port=self._port)

    def execute_sql(self, sql: str, get_rows: bool) -> Optional[Dict]:
        result = {}
        cur = self._conn.cursor()
        cur.execute(sql)
        if get_rows:
            records = cur.fetchall()
            keys = [d[0] for d in cur.description]
            dictionaries = [
                {keys[i]: v for i, v in enumerate(r)} for r in records]
            result['Records'] = dictionaries
        return result

    def execute_chunk_and_write(self, query: str, chunk_size: int, writer_callback) -> List[str]:
        write_locations = []

        with self._conn.cursor() as cursor:
            cursor.execute(query)
            page = 0
            while True:
                rows = cursor.fetchmany(chunk_size)
                if not rows:
                    break
                write_location = writer_callback(page, query, rows)
                write_locations.append(write_location)
                page = page + 1

        return write_locations

