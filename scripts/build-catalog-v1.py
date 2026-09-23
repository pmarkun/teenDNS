#!/usr/bin/env python3
"""Build the first real teenDNS domain catalog from traceable public sources."""

from __future__ import annotations

import csv
import hashlib
import io
import json
import os
import re
import subprocess
import tempfile
from datetime import UTC, datetime
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / "catalog" / "v1"

SPA_URL = (
    "https://www.gov.br/fazenda/pt-br/composicao/orgaos/"
    "secretaria-de-premios-e-apostas/transparencia-ativa-processos-de-"
    "autorizacao-de-apostas-de-quota-fixa/planilha-de-autorizacoes-1.csv"
)
TRACKING_URL = (
    "https://blocklistproject.github.io/Lists/alt-version/tracking-nl.txt"
)
OFCOM_URL = (
    "https://www.ofcom.org.uk/online-safety/protecting-children/"
    "enforcement-programme-to-protect-children-from-encountering-"
    "pornographic-content-through-the-use-of-age-assurance"
)
OFCOM_CATEGORY_URL = (
    "https://www.ofcom.org.uk/online-safety/illegal-and-harmful-content/"
    "register-of-categorised-services-and-list-emerging-category-1-services"
)
EU_ADULT_URL = (
    "https://digital-strategy.ec.europa.eu/en/news/commission-designates-"
    "second-set-very-large-online-platforms-under-digital-services-act"
)

USER_AGENT = "teenDNS catalog builder/1.0 (+https://github.com/)"
DOMAIN_RE = re.compile(
    r"^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+"
    r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$"
)

# Ofcom explicitly identifies these as adult services in its enforcement
# programme. They are kept manually because Ofcom blocks generic HTTP clients
# and does not publish a machine-readable register for this page.
OFCOM_ADULT_DOMAINS = """
4kporn.xxx
429.xxx
ah-me.com
anysex.com
ashemale.one
bdsm.one
bemyhole.com
crazyporn.xxx
empflix.com
eporner.com
fapality.com
fapello.com
ftvgirls.com
ftvmilfs.com
gaygo.tv
gayxo.com
hclips.com
hdzog.com
hdzog.tube
hello.porn
hoes.tube
homo.xxx
hotmovs.com
hqporner.com
imagefap.com
joi.com
justpornflix.com
kemono.cr
love4porn.com
manysex.com
max.porn
motherless.com
moviefap.com
mylust.com
ok.porn
ok.xxx
ooxxx.com
peekvids.com
perfectgirls.xxx
pimpbunny.com
pin.porn
playvids.com
pornflip.com
pornhat.com
pornhat.one
pornhaven.ai
pornl.com
pornoeggs.com
pornrepublic.com
pornstars.tube
porntrex.com
porntrex.tv
scoreland.com
shemale.pub
shemalez.com
shemalez.tube
shesfreaky.com
sunporno.com
thegay.com
thegay.tube
theyarehuge.com
tnaflix.com
tranny.one
tubepornclassic.com
txxx.com
txxx.tube
undress.cc
upornia.com
vjav.com
voyeurhit.com
xcafe.com
xgroovy.com
xxbrits.ch
xxbrits.co.uk
xxbrits.com
xxbrits.cr
xxbrits.lol
xxbrits.mx
xxbrits.party
xxbrits.ru
xxbrits.st
xxbrits.su
xxbrits.to
xxbrits.tube
xxbrits.tv
xxbrits.win
yesvids.com
yourlust.com
""".split()

# The European Commission designated these services as pornographic VLOPs.
EU_ADULT_DOMAINS = ["pornhub.com", "stripchat.com", "xvideos.com", "xnxx.com"]

# Current Category 1 user-to-user services in Ofcom's public register. Domains
# are the canonical public entry points; auxiliary/CDN domains are deliberately
# not inferred in v1.
SOCIAL_DOMAINS = """
facebook.com
instagram.com
pinterest.com
quora.com
reddit.com
roblox.com
snapchat.com
tiktok.com
whatsapp.com
x.com
youtube.com
""".split()

# Legacy canonical hostname retained because the European Commission's first
# DSA designation named the service Twitter and the hostname still resolves.
SOCIAL_LEGACY_DOMAINS = ["twitter.com"]


def fetch(url: str) -> bytes:
    # Use the host curl trust store. The uv-managed Python runtime on this
    # NixOS machine does not inherit the locally installed CA chain.
    result = subprocess.run(
        [
            "curl",
            "--location",
            "--fail",
            "--silent",
            "--show-error",
            "--max-time",
            "60",
            "--user-agent",
            USER_AGENT,
            url,
        ],
        check=True,
        stdout=subprocess.PIPE,
    )
    return result.stdout


def normalize_domain(value: str) -> str:
    value = value.strip().strip(".").replace("\u00a0", "").lower()
    value = value.encode("idna").decode("ascii")
    if not DOMAIN_RE.fullmatch(value):
        raise ValueError(f"invalid domain: {value!r}")
    return value


def parse_spa_domains(payload: bytes) -> list[str]:
    text = payload.decode("utf-8-sig")
    reader = csv.reader(io.StringIO(text), delimiter=";")
    domains: set[str] = set()
    for row in reader:
        if len(row) <= 5 or "." not in row[5]:
            continue
        domains.add(normalize_domain(row[5]))
    return sorted(domains)


def parse_domain_list(payload: bytes) -> tuple[list[str], list[str]]:
    domains: set[str] = set()
    rejected: list[str] = []
    for raw_line in payload.decode("utf-8-sig").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        try:
            domains.add(normalize_domain(line))
        except ValueError:
            rejected.append(line)
    return sorted(domains), rejected


def normalized(domains: list[str]) -> list[str]:
    return sorted({normalize_domain(domain) for domain in domains})


def sha256(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def list_payload(domains: list[str]) -> bytes:
    return ("\n".join(domains) + "\n").encode()


def write_atomic(path: Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as output:
            output.write(payload)
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def main() -> None:
    fetched_at = datetime.now(UTC).replace(microsecond=0).isoformat()
    spa_payload = fetch(SPA_URL)
    tracking_payload = fetch(TRACKING_URL)
    tracking_domains, tracking_rejected = parse_domain_list(tracking_payload)

    lists = {
        "adult-content-regulators.txt": normalized(
            OFCOM_ADULT_DOMAINS + EU_ADULT_DOMAINS
        ),
        "gambling-br-authorized.txt": parse_spa_domains(spa_payload),
        "social-platforms.txt": normalized(
            SOCIAL_DOMAINS + SOCIAL_LEGACY_DOMAINS
        ),
        "tracking-observe.txt": tracking_domains,
    }

    source_manifest = {
        "version": 1,
        "generated_at": fetched_at,
        "policy": {
            "adult-content-regulators.txt": {
                "category": "adult_content",
                "default_action": "block",
                "confidence": "regulator_identified",
            },
            "gambling-br-authorized.txt": {
                "category": "gambling",
                "default_action": "block",
                "confidence": "official_register",
            },
            "social-platforms.txt": {
                "category": "social_platform",
                "default_action": "observe",
                "confidence": "regulator_identified",
            },
            "tracking-observe.txt": {
                "category": "tracking",
                "default_action": "observe",
                "confidence": "community_curated",
            },
        },
        "sources": [
            {
                "id": "spa-mf-authorized-betting",
                "publisher": "Secretaria de Premios e Apostas, Ministerio da Fazenda",
                "url": SPA_URL,
                "retrieved_at": fetched_at,
                "sha256": sha256(spa_payload),
                "output": "gambling-br-authorized.txt",
            },
            {
                "id": "ofcom-adult-enforcement",
                "publisher": "Ofcom",
                "url": OFCOM_URL,
                "retrieved_at": fetched_at,
                "extraction": "manual from named adult services",
                "output": "adult-content-regulators.txt",
            },
            {
                "id": "eu-dsa-adult-vlops",
                "publisher": "European Commission",
                "url": EU_ADULT_URL,
                "retrieved_at": fetched_at,
                "extraction": "manual from designated pornographic VLOPs",
                "output": "adult-content-regulators.txt",
            },
            {
                "id": "ofcom-category-1",
                "publisher": "Ofcom",
                "url": OFCOM_CATEGORY_URL,
                "retrieved_at": fetched_at,
                "extraction": "manual canonical-domain mapping",
                "output": "social-platforms.txt",
            },
            {
                "id": "block-list-project-tracking",
                "publisher": "The Block List Project",
                "url": TRACKING_URL,
                "retrieved_at": fetched_at,
                "sha256": sha256(tracking_payload),
                "license": "Unlicense; downloaded snapshot header states MIT",
                "output": "tracking-observe.txt",
            },
        ],
        "counts": {name: len(domains) for name, domains in lists.items()},
        "rejected": {
            "tracking-observe.txt": {
                "count": len(tracking_rejected),
                "entries": tracking_rejected,
                "reason": "not a valid hostname under the teenDNS v1 grammar",
            }
        },
    }

    for name, domains in lists.items():
        write_atomic(OUTPUT / name, list_payload(domains))

    manifest_payload = (
        json.dumps(source_manifest, indent=2, ensure_ascii=False) + "\n"
    ).encode()
    write_atomic(OUTPUT / "manifest.json", manifest_payload)

    checksum_lines = []
    for name in sorted([*lists, "manifest.json"]):
        payload = (OUTPUT / name).read_bytes()
        checksum_lines.append(f"{sha256(payload)}  {name}")
    write_atomic(
        OUTPUT / "SHA256SUMS", ("\n".join(checksum_lines) + "\n").encode()
    )

    for name, domains in lists.items():
        print(f"{name}: {len(domains)}")


if __name__ == "__main__":
    main()
