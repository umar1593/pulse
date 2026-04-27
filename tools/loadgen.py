#!/usr/bin/env python3
"""Async load generator for the pulse ingest-api.

Usage:
    python3 tools/loadgen.py --rps 1000 --duration 60s

Sends synthetic events at a target throughput and reports p50/p95/p99
latency plus error counts. The numbers feed docs/perf-log.md from week 9.
"""
from __future__ import annotations

import argparse
import asyncio
import random
import re
import statistics
import string
import time
import uuid
from dataclasses import dataclass, field
from typing import List

import httpx

EVENT_TYPES = ["page_view", "click", "purchase", "signup", "logout", "search"]
PATHS = ["/", "/pricing", "/docs", "/blog", "/login", "/signup"]


def _rand_user_id() -> str:
    return "u-" + "".join(random.choices(string.ascii_lowercase + string.digits, k=8))


def _make_event() -> dict:
    return {
        "user_id": _rand_user_id(),
        "event_type": random.choice(EVENT_TYPES),
        "properties": {
            "path": random.choice(PATHS),
            "session_id": str(uuid.uuid4()),
            "ts_client": int(time.time() * 1000),
        },
    }


def _parse_duration(s: str) -> float:
    m = re.fullmatch(r"(\d+)(s|m|h)?", s.strip())
    if not m:
        raise ValueError(f"bad duration: {s}")
    n = int(m.group(1))
    unit = m.group(2) or "s"
    return n * {"s": 1, "m": 60, "h": 3600}[unit]


@dataclass
class Stats:
    latencies_ms: List[float] = field(default_factory=list)
    ok: int = 0
    errors: int = 0

    def report(self) -> str:
        if not self.latencies_ms:
            return f"no successful requests, errors={self.errors}"
        lat = sorted(self.latencies_ms)

        def p(q: float) -> float:
            return lat[min(len(lat) - 1, int(len(lat) * q))]

        return (
            f"ok={self.ok} err={self.errors} "
            f"p50={p(0.50):.1f}ms p95={p(0.95):.1f}ms p99={p(0.99):.1f}ms "
            f"mean={statistics.mean(lat):.1f}ms"
        )


async def _worker(client: httpx.AsyncClient, url: str, queue: asyncio.Queue, stats: Stats) -> None:
    while True:
        payload = await queue.get()
        if payload is None:
            queue.task_done()
            return
        t0 = time.perf_counter()
        try:
            resp = await client.post(url, json=payload, timeout=5.0)
            elapsed = (time.perf_counter() - t0) * 1000
            if 200 <= resp.status_code < 300:
                stats.ok += 1
                stats.latencies_ms.append(elapsed)
            else:
                stats.errors += 1
        except Exception:
            stats.errors += 1
        finally:
            queue.task_done()


async def run(url: str, rps: int, duration_s: float, concurrency: int) -> Stats:
    stats = Stats()
    queue: asyncio.Queue = asyncio.Queue(maxsize=rps * 2)

    async with httpx.AsyncClient(http2=False) as client:
        workers = [
            asyncio.create_task(_worker(client, url, queue, stats))
            for _ in range(concurrency)
        ]

        interval = 1.0 / rps
        end = time.monotonic() + duration_s
        next_send = time.monotonic()
        while time.monotonic() < end:
            await queue.put(_make_event())
            next_send += interval
            sleep_for = next_send - time.monotonic()
            if sleep_for > 0:
                await asyncio.sleep(sleep_for)

        for _ in workers:
            await queue.put(None)
        await queue.join()
        await asyncio.gather(*workers)

    return stats


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--url", default="http://localhost:8080/events")
    p.add_argument("--rps", type=int, default=500)
    p.add_argument("--duration", default="30s")
    p.add_argument("--concurrency", type=int, default=64)
    args = p.parse_args()

    duration_s = _parse_duration(args.duration)
    print(
        f"loadgen: url={args.url} rps={args.rps} duration={duration_s:.0f}s "
        f"concurrency={args.concurrency}"
    )
    stats = asyncio.run(run(args.url, args.rps, duration_s, args.concurrency))
    print(stats.report())


if __name__ == "__main__":
    main()
