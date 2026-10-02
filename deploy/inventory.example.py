# Copy to deploy/inventory.py (ignored by git) and adjust.
#
# Host data:
#   ugubot_dir             where the stack lives (/opt/ugubot)
#   ugubot_user            owner of the files, containers run with its uid/gid
#                          (default: the SSH user)
#   ugubot_web_bind        host address for the web interface (127.0.0.1:8000)
#   ugubot_settings        local settings.toml uploaded on the first deploy
#                          (default: settings.toml.example)
#   ugubot_secrets         local .secrets.toml uploaded on the first deploy
#                          (default: an empty file)
#   ugubot_up              run `docker compose up -d` at the end (False)
#   ugubot_postgres_image  postgres:17-alpine
#   ugubot_nats_image      nats:2.11-alpine
#
# The SSH user needs access to docker (docker group), otherwise run
# pyinfra with --sudo.

hosts = [
    (
        "ugubok.ru",
        {
            "ssh_user": "ugubok",
            "ugubot_web_bind": "127.0.0.1:8000",
            # "ugubot_settings": "settings.toml",
            # "ugubot_secrets": ".secrets.toml",
            # "ugubot_up": True,
        },
    ),
]
