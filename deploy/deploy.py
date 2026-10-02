"""Prepares a host to run the ugubot docker compose stack without sources.

Builds the image locally, uploads it if the host doesn't have it yet, writes
docker-compose.yml and .env, and creates settings.toml / .secrets.toml only
when they are missing. See deploy/README.md.

    pyinfra deploy/inventory.py deploy/deploy.py
"""

import io
import os
import secrets
import shlex
import sys

from pyinfra.context import host
from pyinfra.api.exceptions import DeployError
from pyinfra.facts.files import File, FileContents
from pyinfra.facts.server import Command, User
from pyinfra.operations import files, server

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from local_image import REPO, build_image  # noqa: E402

# Host data (inventory) with defaults.
app_dir = str(host.data.get("ugubot_dir", "/opt/ugubot"))
# Owner of the files; containers run with its uid/gid so settings stay private.
owner = str(host.data.get("ugubot_user") or host.get_fact(User))
web_bind = str(host.data.get("ugubot_web_bind", "127.0.0.1:8000"))
# Local files uploaded on the first deploy; later they are never overwritten.
settings_src = host.data.get("ugubot_settings", os.path.join(REPO, "settings.toml.example"))
secrets_src = host.data.get("ugubot_secrets")
start = str(host.data.get("ugubot_up", False)).lower() in ("1", "true", "yes")  # --data gives strings
postgres_image = host.data.get("ugubot_postgres_image", "postgres:17-alpine")
nats_image = host.data.get("ugubot_nats_image", "nats:2.11-alpine")

if host.get_fact(Command, "docker compose version >/dev/null 2>&1 && echo ok || echo missing") != "ok":
    raise DeployError("docker with the compose plugin is required on the host")

ids = host.get_fact(Command, f"id -u {shlex.quote(owner)} && id -g {shlex.quote(owner)} && id -gn {shlex.quote(owner)}").split()
if len(ids) != 3:
    raise DeployError(f"user {owner!r} does not exist on the host")
uid, gid, group = ids

image = build_image()

files.directory(
    name=f"Create {app_dir}",
    path=app_dir,
    user=owner,
    group=group,
    mode="750",
)

# Upload the image only when the host doesn't have this exact build.
has_image = host.get_fact(Command, f"docker image inspect {image.build_tag} >/dev/null 2>&1 && echo yes || echo no")
if has_image != "yes":
    remote_archive = f"{app_dir}/ugubot-image.tar.gz"
    files.put(
        name=f"Upload {image.tag}",
        src=image.archive,
        dest=remote_archive,
        mode="600",
    )
    server.shell(
        name=f"Load {image.tag}",
        commands=[f"gunzip -c {remote_archive} | docker load && rm -f {remote_archive}"],
    )

files.template(
    name="Write docker-compose.yml",
    src=os.path.join(os.path.dirname(os.path.abspath(__file__)), "templates", "docker-compose.yml.j2"),
    dest=f"{app_dir}/docker-compose.yml",
    user=owner,
    group=group,
    mode="640",
    postgres_image=postgres_image,
    nats_image=nats_image,
)

# .env is rewritten every time, but the generated database password is kept.
existing = dict(
    line.split("=", 1)
    for line in (host.get_fact(FileContents, f"{app_dir}/.env") or [])
    if "=" in line and not line.startswith("#")
)
env = {
    "UGUBOT_IMAGE": image.tag,
    "UGUBOT_UID": uid,
    "UGUBOT_GID": gid,
    "UGUBOT_WEB_BIND": web_bind,
    "POSTGRES_PASSWORD": existing.get("POSTGRES_PASSWORD") or secrets.token_hex(24),
}
files.put(
    name="Write .env",
    src=io.StringIO("# Managed by deploy/deploy.py\n" + "".join(f"{k}={v}\n" for k, v in env.items())),
    dest=f"{app_dir}/.env",
    user=owner,
    group=group,
    mode="600",
)

for name, src in (("settings.toml", settings_src), (".secrets.toml", secrets_src)):
    dest = f"{app_dir}/{name}"
    if host.get_fact(File, dest):
        continue  # never overwrite settings edited on the host
    files.put(
        name=f"Create {name}",
        src=src if src else io.StringIO("# Secrets override settings.toml, same format\n"),
        dest=dest,
        user=owner,
        group=group,
        mode="600",
    )

if start:
    server.shell(
        name="Start the stack",
        commands=[f"cd {app_dir} && docker compose up -d --remove-orphans"],
    )
