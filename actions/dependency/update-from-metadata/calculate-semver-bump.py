#!/usr/bin/env python3
"""Calculate the highest semver bump between two buildpack.toml files.

Usage: calculate-semver-bump.py BEFORE AFTER

Prints one of "major", "minor", "patch", or an empty string when no
dependency version was increased.
"""

import re
import sys
from collections import defaultdict


def parse_dependencies(path):
    try:
        import tomllib
    except ModuleNotFoundError:
        return parse_dependencies_fallback(path)

    try:
        with open(path, "rb") as file:
            config = tomllib.load(file)
    except tomllib.TOMLDecodeError:
        return parse_dependencies_fallback(path)

    return [
        (dependency.get("id"), dependency.get("version"))
        for dependency in config.get("metadata", {}).get("dependencies", [])
    ]


def parse_dependencies_fallback(path):
    """Parse dependency id/version pairs for Python versions without tomllib."""
    dependencies = []
    current = {}
    in_dependencies = False

    section_re = re.compile(r"^\s*\[\[metadata\.dependencies\]\]\s*$")
    id_re = re.compile(r'^\s*id\s*=\s*"([^"]*)"\s*$')
    version_re = re.compile(r'^\s*version\s*=\s*"([^"]*)"\s*$')

    with open(path, encoding="utf-8") as file:
        for line in file:
            if section_re.match(line):
                if current.get("id") and current.get("version"):
                    dependencies.append((current["id"], current["version"]))
                in_dependencies = True
                current = {}
                continue

            if not in_dependencies:
                continue

            if line.lstrip().startswith("["):
                # A nested table, such as licenses, or the next dependency.
                if current.get("id") and current.get("version"):
                    dependencies.append((current["id"], current["version"]))
                in_dependencies = False
                current = {}
                continue

            match = id_re.match(line)
            if match:
                current["id"] = match.group(1)
                continue

            match = version_re.match(line)
            if match:
                current["version"] = match.group(1)

    if current.get("id") and current.get("version"):
        dependencies.append((current["id"], current["version"]))

    return dependencies


def parse_version(version):
    if not version:
        return None

    version = str(version).strip()
    if version.startswith("v"):
        version = version[1:]

    # Ignore pre-release and build metadata, which do not affect the bump.
    version = version.split("+", 1)[0].split("-", 1)[0]

    parts = version.split(".")
    if len(parts) < 2:
        return None

    try:
        return tuple(int(part) for part in parts)
    except ValueError:
        return None


def dependency_groups(dependencies):
    groups = defaultdict(list)
    for dependency_id, version in dependencies:
        parsed = parse_version(version)
        if dependency_id is None or parsed is None:
            continue

        # Group by compatibility line (the major version) so an update to an
        # older line, such as an older LTS runtime, is not hidden by a newer
        # line using a larger major version.
        groups[(dependency_id, parsed[0])].append((parsed, version))

    return groups


def compare_versions(old, new):
    # Pad the tuples so versions with differing component counts compare
    # correctly, e.g. 11.0.32 and 11.0.32.1.
    length = max(len(old), len(new), 3)
    old = tuple(old) + (0,) * (length - len(old))
    new = tuple(new) + (0,) * (length - len(new))

    if new <= old:
        return ""

    if new[0] > old[0]:
        return "major"

    if new[1] > old[1]:
        return "minor"

    return "patch"


def highest_bump(before_path, after_path):
    before = dependency_groups(parse_dependencies(before_path))
    after = dependency_groups(parse_dependencies(after_path))

    rank = {"": 0, "patch": 1, "minor": 2, "major": 3}
    highest = ""

    for key, group in after.items():
        if key not in before:
            # A dependency line that did not exist before is a new major
            # version line.
            bump = "major"
        else:
            before_group = before[key]
            if {version for _, version in before_group} == {
                version for _, version in group
            }:
                bump = ""
            else:
                bump = compare_versions(
                    max(version for version, _ in before_group),
                    max(version for version, _ in group),
                )
                # The core version did not change, but the version string did,
                # for example when only the build metadata changed. Treat the
                # dependency update as a patch.
                if bump == "":
                    bump = "patch"

        if rank[bump] > rank[highest]:
            highest = bump

    return highest


def main():
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} BEFORE AFTER", file=sys.stderr)
        return 2

    print(highest_bump(sys.argv[1], sys.argv[2]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
