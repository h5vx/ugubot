import datetime
import hmac
import secrets


class Signer:
    def __init__(self, key: str, expiration: str = None) -> None:
        self.key = key.encode("utf-8")
        self.expiration_seconds = None

        if expiration:
            seconds_per_unit = {"s": 1, "m": 60, "h": 3600, "d": 86400, "w": 604800}
            self.expiration_seconds = int(expiration[:-1]) * seconds_per_unit[expiration[-1]]

    def get_utcnow_timestamp(self) -> int:
        return int(datetime.datetime.utcnow().timestamp())

    def get_signed_token(self, value=None) -> str:
        if value is None:
            value = str(self.get_utcnow_timestamp())

        signature = hmac.new(self.key, msg=value.encode("utf-8"), digestmod="sha256").hexdigest()
        return ":".join((value, signature))

    def check_token(self, token: str) -> bool:
        try:
            value, signature = token.split(":")

            if not value:
                return False

            signature_encoded = signature.encode("utf-8")
            _, expected_signature = self.get_signed_token(value).encode("utf-8").split(b":")

            # Check the signature itself
            if not secrets.compare_digest(signature_encoded, expected_signature):
                return False

            # Check expiration
            if not self.expiration_seconds:
                return True

            token_issue_time = int(value)

            if self.get_utcnow_timestamp() > token_issue_time + self.expiration_seconds:
                return False
        except ValueError:
            return False

        return True
