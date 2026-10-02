"""Local image build for deploy.py (not named build.py: that clashes with pypa's build).

Imported modules are cached by Python, so the image is built and saved once
per pyinfra run no matter how many hosts are in the inventory.
"""

import functools
import os
import subprocess
from dataclasses import dataclass

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BUILD_DIR = os.path.join(REPO, "deploy", ".build")


@dataclass(frozen=True)
class Image:
    tag: str  # ugubot:<git describe>
    # ugubot:build-<local image id>. Image ids differ between docker image
    # stores (containerd reports the manifest digest), so hosts are checked
    # for this tag instead.
    build_tag: str
    archive: str  # local path to `docker save | gzip`


def _run(*cmd: str) -> str:
    return subprocess.run(cmd, cwd=REPO, check=True, capture_output=True, text=True).stdout.strip()


@functools.cache
def build_image() -> Image:
    version = _run("git", "describe", "--always", "--dirty", "--tags")
    tag = f"ugubot:{version}"

    print(f"--> Building {tag} locally")
    subprocess.run(["docker", "build", "-t", tag, REPO], check=True)

    short_id = _run("docker", "image", "inspect", "-f", "{{.Id}}", tag).removeprefix("sha256:")[:12]
    build_tag = f"ugubot:build-{short_id}"
    _run("docker", "tag", tag, build_tag)

    os.makedirs(BUILD_DIR, exist_ok=True)
    # Named by image id: a rebuild with the same tag (dirty tree) gets a new file.
    archive = os.path.join(BUILD_DIR, f"ugubot-{short_id}.tar.gz")

    if not os.path.exists(archive):
        print(f"--> Saving {tag} to {os.path.relpath(archive, REPO)}")
        tmp = archive + ".tmp"
        with open(tmp, "wb") as out:
            save = subprocess.Popen(["docker", "save", tag, build_tag], stdout=subprocess.PIPE)
            subprocess.run(["gzip", "-1"], stdin=save.stdout, stdout=out, check=True)
            if save.wait() != 0:
                raise RuntimeError("docker save failed")
        os.replace(tmp, archive)

    return Image(tag=tag, build_tag=build_tag, archive=archive)
