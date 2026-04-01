# OTel Exporter Queue Backpressure Guidance

## Purpose

When downstream telemetry endpoints are unavailable, exporter queues can grow and impact droplet stability. This document captures production-standard controls and helps us decide what is mandatory for fleet safety versus optional by workload tier.

## Problem Statement

During downstream outage conditions, queued telemetry may consume memory and, if persistent queue is enabled, disk. If controls are not bounded, this can degrade or exhaust VM resources.

## Eight Production Approaches

1. **Bound queue size**
   - Use a finite `sending_queue.queue_size`.
   - Avoid unbounded in-memory accumulation.

2. **Bound retry duration**
   - Use finite `retry_on_failure.max_elapsed_time`.
   - Prevent infinite retries during long outages.

3. **Prefer data drop over host exhaustion**
   - Once limits are reached, drop telemetry with clear signals.
   - Protect droplet health first.

4. **Enable persistent queue only when durability is required**
   - Use `sending_queue.storage` selectively, not by default for every pipeline.
   - Persistent queue improves durability but increases disk-risk surface.

5. **Enforce host-level disk guardrails**
   - Use controlled storage path, disk alerts, and cleanup policy.
   - Collector config alone is not a complete disk-isolation strategy.

6. **Tune by SLO and ingest profile**
   - Size queue and retry window based on expected traffic and acceptable outage window.
   - Avoid one-size-fits-all defaults for very different workloads.

7. **Monitor collector self-health**
   - Track queue depth, retries, drops, exporter errors, memory, and disk usage.
   - Alert on sustained growth and prolonged downstream failures.

8. **Run regular outage drills**
   - Test endpoint blackhole, DNS/TLS failures, and long outage scenarios.
   - Validate bounded behavior and safe recovery after endpoint restoration.

## Decision Framework For Our Fleet

We should explicitly decide three categories:

- **Mandatory (cannot compromise):** controls required on every droplet.
- **Recommended (phase-in):** strong defaults we should adopt broadly.
- **Optional (tier-specific):** controls enabled only for specific workloads.

### Suggested Mandatory Baseline

- Finite queue size.
- Finite retry window.
- Clear drop behavior once limits are reached.
- Collector self-monitoring with alert thresholds.

### Suggested Tier-Specific Controls

- Persistent queue only where delivery durability is required.
- Stricter disk isolation/quota strategy for high-risk workloads.
- Per-tier queue/retry values for low/medium/high ingest profiles.

## Compromise Guidelines

These tradeoffs must be defined by us before fleet rollout:

- **Durability vs host safety:** larger buffers reduce loss, but increase resource pressure.
- **Outage tolerance vs freshness:** longer retries increase delivery chance, but hold stale data longer.
- **Uniform defaults vs tiering:** simpler operations vs safer workload-specific tuning.

## What We Need To Finalize

1. Fleet-wide default `queue_size`.
2. Fleet-wide default `max_elapsed_time`.
3. Whether persistent queue is default-off or default-on.
4. Disk guardrail policy for queue storage path.
5. Alert thresholds and escalation policy.
6. Accepted compromise boundaries under prolonged outage.

## Recommended Next Step

Run controlled outage tests on representative droplets, then finalize:

- baseline mandatory settings,
- tier overrides,
- and an operations runbook for endpoint outage scenarios.
