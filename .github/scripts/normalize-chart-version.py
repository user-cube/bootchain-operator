#!/usr/bin/env python3
"""
Strip optional 'v' prefix from version and appVersion in a Helm Chart.yaml.
Chart version and appVersion must be semver without prefix (e.g. 1.0.0) for OCI and Artifact Hub.

Usage:
  python normalize-chart-version.py <path-to-Chart.yaml>
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser(description="Strip 'v' prefix from Chart.yaml version and appVersion")
    parser.add_argument("chart_yaml", type=Path, help="Path to Chart.yaml")
    args = parser.parse_args()

    path = args.chart_yaml.resolve()
    if not path.exists():
        raise SystemExit(f"File not found: {path}")

    text = path.read_text(encoding="utf-8")
    original = text
    # Strip leading 'v' from version: and appVersion: lines
    text = re.sub(r"^(\s*version:\s*)v([\d.]+\S*)\s*$", r"\1\2", text, flags=re.MULTILINE)
    text = re.sub(r"^(\s*appVersion:\s*)v([\d.]+\S*)\s*$", r"\1\2", text, flags=re.MULTILINE)
    if text != original:
        path.write_text(text, encoding="utf-8")
        print(f"Normalized version/appVersion in {path}")
    else:
        print(f"No change needed in {path}")


if __name__ == "__main__":
    main()
