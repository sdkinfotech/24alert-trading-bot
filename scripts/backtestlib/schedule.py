"""FORTS trading schedule (Europe/Moscow)."""
from datetime import timedelta, timezone

try:
    from zoneinfo import ZoneInfo
    MSK = ZoneInfo("Europe/Moscow")
except Exception:
    MSK = timezone(timedelta(hours=3))  # fallback without tzdata (e.g. Windows)
TRADING_WINDOWS = [
    ("forts_day_before_clearing", 10 * 60, 14 * 60),
    ("forts_day_after_clearing", 14 * 60 + 5, 18 * 60 + 50),
    ("forts_evening", 19 * 60, 23 * 60 + 50),
]


def is_main_session(t) -> bool:
    local = t.astimezone(MSK)
    if local.weekday() >= 5:
        return False
    minutes = local.hour * 60 + local.minute
    return any(start <= minutes < end for _, start, end in TRADING_WINDOWS)


def schedule_info() -> dict:
    return {
        "timezone": "Europe/Moscow",
        "sessions": [
            {
                "name": name,
                "start": f"{start // 60:02d}:{start % 60:02d}",
                "end": f"{end // 60:02d}:{end % 60:02d}",
            }
            for name, start, end in TRADING_WINDOWS
        ],
        "weekdays_only": True,
    }
