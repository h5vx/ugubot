import logging
import pickle
import typing as t

from config import settings

logger = logging.getLogger(__name__)

try:
    import redis
except ImportError:
    logger.error("Redis package isn't installed")
    redis = None

if redis and settings.redis.enabled:
    try:
        r = redis.Redis(
            host=settings.redis.host,
            port=settings.redis.port,
            db=settings.redis.db,
        )
    except redis.ConnectionError as e:
        logger.error("Couldn't connect to redis: {e}")
        r = None


class cache:
    available = bool(r)

    @staticmethod
    def set(key: str, value: object) -> None:
        value = pickle.dumps(value)
        r.set(key, value)

    @staticmethod
    def get(key: str) -> object:
        value = r.get(key)

        if value is None:
            return None

        return pickle.loads(value)

    @staticmethod
    def scan_keys(match: str) -> t.Generator[str, None, None]:
        for key in r.scan_iter(match):
            yield key.decode("ascii")


__all__ = [cache]
