#!/usr/bin/env python3
"""Build the first real teenDNS domain catalog from traceable public sources."""

from __future__ import annotations

import argparse
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
PHISHING_URL = (
    "https://blocklistproject.github.io/Lists/alt-version/phishing-nl.txt"
)
RANSOMWARE_URL = (
    "https://blocklistproject.github.io/Lists/alt-version/ransomware-nl.txt"
)
OFCOM_URL = (
    "https://www.ofcom.org.uk/online-safety/protecting-children/"
    "enforcement-programme-to-protect-children-from-encountering-"
    "pornographic-content-through-the-use-of-age-assurance"
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

# Service pools are intentionally small sets of provider-specific DNS suffixes.
# A pool is useful for a family choice such as "pause TikTok"; it is not a
# content rating or a recommendation to block the service. Shared infrastructure
# is documented separately and never enters the generated blocking list.
SERVICE_POOLS = {
    "discord": {
        "label": "Discord",
        "theme": "messaging_and_communities",
        "domains": [
            "discord.com",
            "discord.gg",
            "discord.media",
            "discord.gift",
            "discordapp.com",
            "discordapp.net",
            "dis.gd",
        ],
        "shared_dependencies": ["googleapis.com", "gstatic.com"],
        "evidence_urls": [
            "https://support.discord.com/hc/en-us/articles/360042987951-Discordapp-com-is-now-Discord-com",
            "https://discord.com/developers/docs/reference",
        ],
    },
    "facebook": {
        "label": "Facebook",
        "theme": "social_platforms",
        "domains": ["facebook.com", "fb.com"],
        "shared_dependencies": [
            "facebook.net",
            "fbcdn.net",
            "fbsbx.com",
        ],
        "evidence_urls": [
            "https://developers.facebook.com/docs/graph-api/overview/",
            "https://www.facebook.com/help/",
        ],
    },
    "instagram": {
        "label": "Instagram",
        "theme": "social_platforms",
        "domains": ["instagram.com", "cdninstagram.com", "ig.me"],
        "shared_dependencies": [
            "facebook.com",
            "facebook.net",
            "fbcdn.net",
            "fbsbx.com",
        ],
        "evidence_urls": [
            "https://developers.facebook.com/docs/instagram-platform/",
            "https://help.instagram.com/",
        ],
    },
    "reddit": {
        "label": "Reddit",
        "theme": "social_platforms",
        "domains": [
            "reddit.com",
            "redd.it",
            "redditmedia.com",
            "redditspace.com",
            "redditstatic.com",
        ],
        "shared_dependencies": ["fastly.net"],
        "evidence_urls": [
            "https://developers.reddit.com/docs/capabilities/server/splash-screen",
            "https://support.reddithelp.com/",
        ],
    },
    "roblox": {
        "label": "Roblox",
        "theme": "games_and_social_play",
        "domains": ["roblox.com", "rbxcdn.com", "robloxapi.com"],
        "shared_dependencies": [
            "amazonaws.com",
            "cloudfront.net",
            "googleapis.com",
        ],
        "evidence_urls": [
            "https://create.roblox.com/docs/cloud/open-cloud",
            "https://en.help.roblox.com/hc/en-us/articles/203312840-Firewall-and-Router-Issues",
        ],
    },
    "snapchat": {
        "label": "Snapchat",
        "theme": "social_platforms",
        "domains": [
            "snapchat.com",
            "snap.com",
            "snapkit.com",
            "sc-cdn.net",
        ],
        "shared_dependencies": ["googleapis.com", "gstatic.com"],
        "evidence_urls": [
            "https://developers.snap.com/",
            "https://developers.snap.com/marketing-api/Ads-API/dynamic-product-ads",
        ],
    },
    "telegram": {
        "label": "Telegram",
        "theme": "messaging_and_communities",
        "domains": ["telegram.org", "telegram.me", "t.me", "telesco.pe"],
        "shared_dependencies": [],
        "evidence_urls": [
            "https://core.telegram.org/api/links",
            "https://core.telegram.org/bots/api",
        ],
    },
    "threads": {
        "label": "Threads",
        "theme": "social_platforms",
        "domains": ["threads.com", "threads.net"],
        "shared_dependencies": [
            "facebook.com",
            "facebook.net",
            "fbcdn.net",
            "fbsbx.com",
            "instagram.com",
        ],
        "evidence_urls": [
            "https://developers.facebook.com/docs/threads/",
            "https://help.instagram.com/769983657850450",
        ],
    },
    "tiktok": {
        "label": "TikTok",
        "theme": "social_video",
        "domains": [
            "tiktok.com",
            "tiktokapis.com",
            "tiktokcdn.com",
            "tiktokcdn-eu.com",
            "tiktokv.com",
            "muscdn.com",
            "musical.ly",
        ],
        "shared_dependencies": [
            "akamaized.net",
            "byteimg.com",
            "byteoversea.com",
            "byteoversea.net",
            "ibyteimg.com",
            "ibytedtos.com",
            "pstatp.com",
        ],
        "evidence_urls": [
            "https://developers.tiktok.com/doc/content-posting-api-get-started/",
            "https://support.tiktok.com/",
        ],
    },
    "twitch": {
        "label": "Twitch",
        "theme": "social_video",
        "domains": ["twitch.tv", "twitchcdn.net", "jtvnw.net"],
        "shared_dependencies": ["amazonaws.com", "cloudfront.net"],
        "evidence_urls": [
            "https://dev.twitch.tv/docs/embed/",
            "https://help.twitch.tv/",
        ],
    },
    "whatsapp": {
        "label": "WhatsApp",
        "theme": "messaging_and_communities",
        "domains": ["whatsapp.com", "whatsapp.net"],
        "shared_dependencies": [
            "facebook.com",
            "facebook.net",
            "fbcdn.net",
            "fbsbx.com",
        ],
        "evidence_urls": [
            "https://developers.facebook.com/docs/whatsapp/",
            "https://faq.whatsapp.com/",
        ],
    },
    "x": {
        "label": "X",
        "theme": "social_platforms",
        "domains": ["x.com", "twitter.com", "t.co", "twimg.com"],
        "shared_dependencies": [],
        "evidence_urls": [
            "https://developer.x.com/en/docs/x-for-websites/overview",
            "https://help.x.com/en/using-x/x-urls",
        ],
    },
    "youtube": {
        "label": "YouTube",
        "theme": "social_video",
        "domains": [
            "youtube.com",
            "youtube-nocookie.com",
            "youtu.be",
            "ytimg.com",
            "googlevideo.com",
            "youtubei.googleapis.com",
        ],
        "shared_dependencies": [
            "google.com",
            "googleapis.com",
            "googleusercontent.com",
            "gstatic.com",
            "ggpht.com",
        ],
        "evidence_urls": [
            "https://support.google.com/youtube/answer/171780",
            "https://support.google.com/a/answer/6334001",
        ],
    },
}


PRESETS = {
    "version": 1,
    "principles": [
        "A escolha e do domicilio; o preset e apenas um ponto de partida.",
        "Nenhum preset exige ou armazena data de nascimento.",
        "Bloquear um servico nao equivale a julgar quem o utiliza.",
        "Mensageria, estudo, criacao, comunidade e jogo podem coexistir no mesmo servico.",
    ],
    "presets": [
        {
            "id": "acompanhado",
            "label": "Acompanhado",
            "description": "Mais escolhas combinadas antes de liberar servicos mistos.",
            "themes": {
                "adult_content": "block",
                "gambling": "block",
                "social_platforms": "block",
                "social_video": "block",
                "messaging_and_communities": "observe",
                "games_and_social_play": "observe",
            },
            "service_overrides": {},
        },
        {
            "id": "explorando",
            "label": "Explorando",
            "description": "Acesso amplo com visibilidade e acordos por servico.",
            "themes": {
                "adult_content": "block",
                "gambling": "block",
                "social_platforms": "observe",
                "social_video": "observe",
                "messaging_and_communities": "observe",
                "games_and_social_play": "observe",
            },
            "service_overrides": {},
        },
        {
            "id": "autonomia-guiada",
            "label": "Autonomia guiada",
            "description": "Protecoes essenciais e conversa a partir do uso observado.",
            "themes": {
                "adult_content": "block",
                "gambling": "block",
                "social_platforms": "allow",
                "social_video": "allow",
                "messaging_and_communities": "allow",
                "games_and_social_play": "allow",
            },
            "service_overrides": {},
        },
    ],
}


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


def validate_service_pools() -> None:
    known_actions = {"allow", "observe", "block"}
    claimed: dict[str, str] = {}
    for service_id, pool in SERVICE_POOLS.items():
        if not re.fullmatch(r"[a-z0-9-]+", service_id):
            raise ValueError(f"invalid service id: {service_id!r}")
        domains = normalized(pool["domains"])
        if len(domains) != len(pool["domains"]):
            raise ValueError(f"duplicate domain in service pool: {service_id}")
        for domain in domains:
            previous = claimed.setdefault(domain, service_id)
            if previous != service_id:
                raise ValueError(
                    f"{domain} is exclusive to both {previous} and {service_id}"
                )
        for domain in pool["shared_dependencies"]:
            normalize_domain(domain)
        if not pool["evidence_urls"]:
            raise ValueError(f"service pool without evidence: {service_id}")

    for preset in PRESETS["presets"]:
        actions = set(preset["themes"].values())
        if not actions <= known_actions:
            raise ValueError(f"unknown action in preset {preset['id']}: {actions}")


def check_current_catalog() -> None:
    checksum_path = OUTPUT / "SHA256SUMS"
    for line in checksum_path.read_text().splitlines():
        expected, relative = line.split(maxsplit=1)
        path = OUTPUT / relative
        actual = sha256(path.read_bytes())
        if actual != expected:
            raise ValueError(f"checksum mismatch: {relative}")

    manifest = json.loads((OUTPUT / "manifest.json").read_text())
    for relative, expected_count in manifest["counts"].items():
        path = OUTPUT / relative
        values = path.read_text().splitlines()
        if values != sorted(set(values)):
            raise ValueError(f"list is not sorted and unique: {relative}")
        for value in values:
            normalize_domain(value)
        if len(values) != expected_count:
            raise ValueError(
                f"count mismatch for {relative}: {len(values)} != {expected_count}"
            )
        print(f"{relative}: {len(values)}")

    pools = json.loads((OUTPUT / "service-pools.json").read_text())
    for service_id, pool in pools["services"].items():
        values = (OUTPUT / pool["domain_source"]).read_text().splitlines()
        if values != pool["domains"]:
            raise ValueError(f"service pool mismatch: {service_id}")

    presets = json.loads((OUTPUT / "presets.json").read_text())
    known_actions = {"allow", "observe", "block"}
    for preset in presets["presets"]:
        if not set(preset["themes"].values()) <= known_actions:
            raise ValueError(f"invalid preset actions: {preset['id']}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--check",
        action="store_true",
        help="validate the committed catalog without downloading or rewriting it",
    )
    arguments = parser.parse_args()
    if arguments.check:
        check_current_catalog()
        return

    fetched_at = datetime.now(UTC).replace(microsecond=0).isoformat()
    validate_service_pools()
    spa_payload = fetch(SPA_URL)
    phishing_payload = fetch(PHISHING_URL)
    ransomware_payload = fetch(RANSOMWARE_URL)
    phishing_domains, phishing_rejected = parse_domain_list(phishing_payload)
    ransomware_domains, ransomware_rejected = parse_domain_list(
        ransomware_payload
    )

    lists = {
        "adult-content-regulators.txt": normalized(
            OFCOM_ADULT_DOMAINS + EU_ADULT_DOMAINS
        ),
        "gambling-br-authorized.txt": parse_spa_domains(spa_payload),
        "security-threats.txt": sorted(
            set(phishing_domains) | set(ransomware_domains)
        ),
    }
    for service_id, pool in SERVICE_POOLS.items():
        lists[f"services/{service_id}.txt"] = normalized(pool["domains"])

    service_pools = {
        "version": 1,
        "generated_at": fetched_at,
        "matching": "domain suffix, including the apex itself",
        "default_action": "observe",
        "warning": (
            "Pools are best-effort service controls, not content ratings. "
            "Shared dependencies are excluded to reduce collateral blocking."
        ),
        "services": {
            service_id: {
                "label": pool["label"],
                "theme": pool["theme"],
                "domain_source": f"services/{service_id}.txt",
                "domains": normalized(pool["domains"]),
                "shared_dependencies_excluded": normalized(
                    pool["shared_dependencies"]
                ),
                "evidence_urls": pool["evidence_urls"],
                "confidence": "official_docs_manual_mapping",
            }
            for service_id, pool in sorted(SERVICE_POOLS.items())
        },
    }
    metadata_files = {
        "service-pools.json": (
            json.dumps(service_pools, indent=2, ensure_ascii=False) + "\n"
        ).encode(),
        "presets.json": (
            json.dumps(PRESETS, indent=2, ensure_ascii=False) + "\n"
        ).encode(),
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
            "security-threats.txt": {
                "category": "security_threats",
                "default_action": "block",
                "confidence": "community_curated",
                "scope": "phishing and ransomware",
            },
            **{
                f"services/{service_id}.txt": {
                    "category": pool["theme"],
                    "service": service_id,
                    "default_action": "observe",
                    "confidence": "official_docs_manual_mapping",
                }
                for service_id, pool in sorted(SERVICE_POOLS.items())
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
                "id": "block-list-project-phishing",
                "publisher": "The Block List Project",
                "url": PHISHING_URL,
                "retrieved_at": fetched_at,
                "sha256": sha256(phishing_payload),
                "license": "MIT",
                "output": "security-threats.txt",
            },
            {
                "id": "block-list-project-ransomware",
                "publisher": "The Block List Project",
                "url": RANSOMWARE_URL,
                "retrieved_at": fetched_at,
                "sha256": sha256(ransomware_payload),
                "license": "MIT",
                "output": "security-threats.txt",
            },
            {
                "id": "official-service-documentation",
                "publisher": "Service providers",
                "retrieved_at": fetched_at,
                "extraction": (
                    "manual provider-specific suffix mapping; see "
                    "service-pools.json evidence_urls"
                ),
                "license": "factual domain identifiers only; no list copied",
                "outputs": sorted(
                    f"services/{service_id}.txt"
                    for service_id in SERVICE_POOLS
                ),
            },
        ],
        "counts": {name: len(domains) for name, domains in lists.items()},
        "rejected": {
            "security-threats.txt": {
                "count": len(phishing_rejected) + len(ransomware_rejected),
                "entries": sorted(
                    set(phishing_rejected) | set(ransomware_rejected)
                ),
                "reason": "not a valid hostname under the teenDNS v1 grammar",
            },
        },
    }

    for name, domains in lists.items():
        write_atomic(OUTPUT / name, list_payload(domains))
    for name, payload in metadata_files.items():
        write_atomic(OUTPUT / name, payload)

    manifest_payload = (
        json.dumps(source_manifest, indent=2, ensure_ascii=False) + "\n"
    ).encode()
    write_atomic(OUTPUT / "manifest.json", manifest_payload)

    checksum_lines = []
    for name in sorted([*lists, *metadata_files, "manifest.json"]):
        payload = (OUTPUT / name).read_bytes()
        checksum_lines.append(f"{sha256(payload)}  {name}")
    write_atomic(
        OUTPUT / "SHA256SUMS", ("\n".join(checksum_lines) + "\n").encode()
    )

    for name, domains in lists.items():
        print(f"{name}: {len(domains)}")


if __name__ == "__main__":
    main()
