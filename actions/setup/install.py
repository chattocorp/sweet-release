"""Install a verified release archive without requiring a Go toolchain."""

import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import tarfile
import tempfile
import zipfile


REPOSITORY = "chattocorp/sweet-release"


def asset_name(version, system, machine):
    operating_system = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}[system]
    architecture = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}[machine.lower()]
    extension = "zip" if operating_system == "windows" else "tar.gz"
    return f"sweet_{version}_{operating_system}_{architecture}.{extension}"


def extract_binary(archive, checksums, destination):
    entries = [line.split() for line in checksums.read_text().splitlines()]
    matches = [parts[0] for parts in entries if len(parts) == 2 and parts[1].lstrip("*") == archive.name]
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    if matches != [digest]:
        raise ValueError(f"Missing, duplicate, or incorrect checksum for {archive.name}")
    # Extract only the executable. Archive paths cannot write outside destination.
    binary = "sweet.exe" if archive.suffix == ".zip" else "sweet"
    if archive.suffix == ".zip":
        with zipfile.ZipFile(archive) as bundle:
            content = bundle.read(binary)
    else:
        with tarfile.open(archive) as bundle:
            member = bundle.getmember(binary)
            if not member.isfile():
                raise ValueError("Release executable must be a regular file")
            content = bundle.extractfile(member).read()
    destination.mkdir(parents=True, exist_ok=True)
    target = destination / binary
    target.write_bytes(content)
    target.chmod(0o755)
    return target


def main():
    requested = os.environ.get("SWEET_VERSION", "latest")
    if requested != "latest" and (not requested or requested.startswith("-") or "\n" in requested):
        raise ValueError("version must be a release tag or latest")
    command = ["gh", "release", "view", "--repo", REPOSITORY, "--json", "tagName"]
    if requested != "latest":
        command.append(requested)
    tag = json.loads(subprocess.check_output(command, text=True))["tagName"]
    match = re.fullmatch(r"(?:sweet@|apps/sweet/v|pkg/sweet/v|pkg/cli/v)([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.+-]+)?)", tag)
    if not match:
        raise ValueError(f"Unsupported Sweet release tag: {tag}")
    version = match[1]
    name = asset_name(version, platform.system(), platform.machine())
    with tempfile.TemporaryDirectory(prefix="sweet-download-") as temporary:
        subprocess.run(["gh", "release", "download", tag, "--repo", REPOSITORY,
                        "--pattern", name, "--pattern", "checksums.txt", "--dir", temporary], check=True)
        destination = Path(tempfile.mkdtemp(prefix="sweet-bin-", dir=os.environ["RUNNER_TEMP"]))
        binary = extract_binary(Path(temporary) / name, Path(temporary) / "checksums.txt", destination)
    actual = subprocess.check_output([str(binary), "--version"], text=True).strip()
    if actual != f"sweet {version}":
        raise ValueError(f"Unexpected executable version: {actual}")
    with open(os.environ["GITHUB_PATH"], "a") as output:
        output.write(f"{destination}\n")
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write(f"version={version}\ntag={tag}\n")
    print(f"Installed {actual} from {tag}")


if __name__ == "__main__":
    main()
