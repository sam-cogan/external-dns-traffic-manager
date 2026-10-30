#!/usr/bin/env python3
"""Fail on drift between the support definition, modules, and deployment pins."""
import json
from pathlib import Path
import re
import sys


def check(root):
    matrix = json.loads((root / "compatibility.json").read_text())
    problems = []
    module = (root / "go.mod").read_text()
    if f'go {matrix["go"]}\n' not in module:
        problems.append("go.mod toolchain differs from compatibility.json")
    for package in ("api", "apimachinery", "client-go"):
        if not re.search(rf"k8s.io/{package} {re.escape(matrix['clientGo'])}(?:\s|$)", module):
            problems.append(f"k8s.io/{package} must be {matrix['clientGo']}")
    for package, version in matrix["azure"].items():
        if not re.search(rf"/{package} {re.escape(version)}(?:\s|$)", module):
            problems.append(f"Azure {package} must be {version}")
    docker = (root / "Dockerfile").read_text()
    docker_from = [line.removeprefix("FROM ").split(" AS ", 1)[0] for line in docker.splitlines() if line.startswith("FROM ")]
    if docker_from != [matrix["docker"]["builder"], matrix["docker"]["runtime"]]:
        problems.append("Docker base images differ from compatibility.json")
    workflow = (root / ".github/workflows/ci.yml").read_text()
    for version in matrix["kubernetes"]:
        kubectl = version["kubectl"]
        if kubectl != f'v{version["version"]}':
            problems.append(f'Kubernetes {version["version"]} must use matching kubectl {kubectl}')
        if f'kubectl: {kubectl}' not in workflow or f'image: {version["image"]}' not in workflow:
            problems.append(f'Kubernetes {version["version"]} CI pins differ from compatibility.json')
    for path in [*root.glob("examples/**/*.yaml"), *root.glob("deploy/**/*.sh")]:
        text = path.read_text()
        for version in re.findall(r"registry.k8s.io/external-dns/external-dns:([^\s]+)", text):
            if version != matrix["defaultExternalDNS"]:
                problems.append(f"{path}: ExternalDNS {version} differs from default")
        if "external-dns.alpha.kubernetes.io/" in text:
            problems.append(f"{path}: legacy annotation prefix is not enabled on supported ExternalDNS")
    for version in matrix["externalDNS"]:
        path = root / "tests/contract" / ("go.mod" if version == matrix["defaultExternalDNS"] else f"{version}.mod")
        if f"sigs.k8s.io/external-dns {version}\n" not in path.read_text():
            problems.append(f"{path}: missing exact upstream contract dependency")
        if not path.with_suffix(".sum").is_file():
            problems.append(f"{path}: missing dependency lock")
    if not matrix["externalDNS"] or not matrix["kubernetes"]:
        problems.append("support matrices must not be empty")
    return problems


if __name__ == "__main__":
    errors = check(Path(__file__).resolve().parents[1])
    if errors:
        sys.exit("\n".join(errors))
    print("Compatibility pins are consistent")
