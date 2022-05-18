import logging
import traceback

from pythonjsonlogger import jsonlogger
from typing import Any, Dict, Optional


class Logger:
    def __init__(self, logger: logging.Logger):
        self._last_set_trace_id: Optional[str] = None
        self._logger = logger

    def set_trace_id(self, context: Any) -> None:
        if not context:
            return
        if not getattr(context, 'aws_request_id'):
            return
        self._last_set_trace_id = context.aws_request_id

    def info(self, msg: str, extra: Dict[str, str] = {}) -> None:
        if extra is None:
            extra = {}
        extra['trace_id'] = self._last_set_trace_id
        self._logger.info(msg, extra=extra)

    def error(self, err_msg: str, extra: Dict[str, str] = {}) -> None:
        if extra is None:
            extra = {}
        extra['stacktrace'] = traceback.format_exc()
        extra['trace_id'] = self._last_set_trace_id
        self._logger.error(err_msg, extra=extra)

    @classmethod
    def make_logger(cls, name: str, level=logging.INFO):
        handler = logging.StreamHandler()
        format_str = '%(message)%(levelname)%(name)%(asctime)'
        formatter = jsonlogger.JsonFormatter(format_str)
        handler.setFormatter(formatter)
        logger = logging.getLogger(name)
        logger.addHandler(handler)
        logger.setLevel(level)
        logger.propagate = False

        return cls(logger)

