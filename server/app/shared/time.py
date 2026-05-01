from datetime import datetime, timedelta, timezone
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

from .config import get_settings


def app_timezone():
    timezone_name = get_settings().app_timezone
    try:
        return ZoneInfo(timezone_name)
    except ZoneInfoNotFoundError:
        if timezone_name == "Asia/Shanghai":
            return timezone(timedelta(hours=8), name=timezone_name)
        return datetime.now().astimezone().tzinfo or timezone.utc


def local_timestamp() -> str:
    return datetime.now(app_timezone()).strftime("%Y-%m-%d %H:%M:%S")
