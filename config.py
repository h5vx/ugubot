from dynaconf import Dynaconf
import logging
import logging.config

settings = Dynaconf(
    envvar_prefix="UGUBOT",
    settings_files=['settings.toml', '.secrets.toml'],
    merge_enabled=True,
)

# `envvar_prefix` = export envvars with `export DYNACONF_FOO=bar`.
# `settings_files` = Load these files in the order.

logging.config.dictConfig(settings.logging)