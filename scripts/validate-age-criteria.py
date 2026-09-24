#!/usr/bin/env python3
"""Validate the internal consistency of the teenDNS age-rating artifacts."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
CRITERIA = ROOT / "criteria"
BANDS = ["L", "6", "10", "12", "14", "16", "18"]
AXES = {"violence", "sex_nudity", "drugs", "interactivity"}
CRITERION_ID = re.compile(r"^[A-D]\.[1-7]\.[0-9]+$")


def load(name: str) -> dict[str, Any]:
    with (CRITERIA / name).open(encoding="utf-8") as source:
        return json.load(source)


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def main() -> None:
    taxonomy = load("age-rating-v1.json")
    profile = load("profile-12-13-v1.json")
    guidance = load("priority-guidance-12-13-v1.json")
    result_schema = load("classification-result.schema.json")

    require(taxonomy["schema_version"] == "1.0.0", "unexpected taxonomy version")
    require([band["code"] for band in taxonomy["bands"]] == BANDS, "invalid band order")
    require({axis["code"] for axis in taxonomy["axes"]} == AXES, "invalid axes")

    all_criteria: dict[str, tuple[str, str]] = {}
    priority_ids: set[str] = set()
    for axis in taxonomy["axes"]:
        require(list(axis["bands"]) == BANDS, f"invalid bands for {axis['code']}")
        for band, criteria in axis["bands"].items():
            for criterion in criteria:
                criterion_id = criterion["id"]
                require(CRITERION_ID.fullmatch(criterion_id) is not None, f"invalid id {criterion_id}")
                require(criterion_id not in all_criteria, f"duplicate id {criterion_id}")
                require(criterion["label"].strip() != "", f"empty label for {criterion_id}")
                all_criteria[criterion_id] = (axis["code"], band)
                if band in {"14", "16", "18"}:
                    priority_ids.add(criterion_id)

    require(len(all_criteria) == 106, f"expected 106 criteria, found {len(all_criteria)}")
    require(len(priority_ids) == 52, f"expected 52 priority criteria, found {len(priority_ids)}")

    guidance_ids: set[str] = set()
    for group in guidance["guidance"]:
        rating = group["rating"]
        require(rating in {"14", "16", "18"}, f"invalid guidance rating {rating}")
        for criterion in group["criteria"]:
            criterion_id = criterion["id"]
            require(criterion_id not in guidance_ids, f"duplicate guidance id {criterion_id}")
            require(criterion_id in all_criteria, f"unknown guidance id {criterion_id}")
            require(all_criteria[criterion_id][1] == rating, f"rating mismatch for {criterion_id}")
            require(criterion["definition"].strip() != "", f"empty definition for {criterion_id}")
            require(len(criterion["signals"]) >= 2, f"too few signals for {criterion_id}")
            require(all(signal.strip() for signal in criterion["signals"]), f"empty signal for {criterion_id}")
            guidance_ids.add(criterion_id)

    missing = priority_ids - guidance_ids
    extra = guidance_ids - priority_ids
    require(not missing, f"priority guidance missing ids: {sorted(missing)}")
    require(not extra, f"priority guidance has extra ids: {sorted(extra)}")

    require(profile["target_ages"] == [12, 13], "invalid target ages")
    require(profile["maximum_compatible_rating"] == 12, "invalid compatible rating")
    actions = {item["default_action"] for item in profile["decision_bands"]}
    require(actions == {"allow", "mediate", "protect", "observe"}, "invalid action set")

    require(
        result_schema["$schema"] == "https://json-schema.org/draft/2020-12/schema",
        "unexpected JSON Schema dialect",
    )
    require(
        result_schema["properties"]["rating"]["enum"]
        == ["L", "6", "10", "12", "14", "16", "18", "unknown"],
        "schema rating enum does not match taxonomy",
    )

    print("age-rating artifacts: ok")
    print(f"criteria={len(all_criteria)} priority={len(priority_ids)} axes={len(AXES)} bands={len(BANDS)}")


if __name__ == "__main__":
    main()
