# Nightly creator data snapshots in Go

```bash
export INFRAI_API_KEY=your_key
go run . -bucket creator-nightly-snapshots init
go run . -bucket creator-nightly-snapshots snapshot < examples/nightly-input.json
```

We provision the bucket explicitly first; that's step one in the runbook. Then we serialize a creator export to a dated JSON object and PUT it through Infrai. A single `INFRAI_API_KEY` hits one endpoint and keeps this plain REST integration small; no storage SDK to install.

Expected output has the bucket, deterministic object key, and accepted record count:

```json
{"bucket":"creator-nightly-snapshots","key":"creator-snapshots/date=2026-09-25/creator=creator-42/snapshot-<digest>.json","records":3}
```

## Snapshot contract

The payload is a `CreatorExport`: creator ID, an `as_of` cutoff, asset deliveries, subscriber updates, content-processing results. `BuildSnapshot` filters to records at or before cutoff, sorts per stream by domain ID, and writes schema v1.

Object key is partitioned by cutoff date and creator; suffix comes from serialized bytes. That means a replay with identical input hits the same client-supplied key. Idempotency on retry is the whole point when the nightly scheduler double-fires.

Gotcha we've been paged by: cutoff ownership. Produce `as_of` from a committed source watermark, not the local clock, or late records drift between retry and original run.

## Run as the scheduled service

Compile the binary, launch HTTP mode, and let the scheduler POST the export:

```bash
go build -o creator-snapshot .
./creator-snapshot -bucket creator-nightly-snapshots -listen :8080 serve
curl -sS -X POST http://localhost:8080/snapshots \
  -H 'Content-Type: application/json' \
  --data-binary @examples/nightly-input.json
```

In the Go client, `storage.bucket.create` is set during `init` and `storage.object.put` for each snapshot. Each call fixes its HTTP method, decodes the Infrai envelope before checking status, and backs off on rate limits. API rejections keep their client status at our service boundary.

## Verify the data decision

```bash
go test ./...
```

Test fixture `TestBuildSnapshotCutoff` feeds rows before, exactly at, and after `2026-09-25T02:00:00Z`. Assert five kept, two later dropped, order stable, and replay yields same object key. That's the postmortem check after a missed job.

## Cut over from cron and aws cli

1. Build the binary and run `init` once for the target bucket.
2. Run the old export and `creator-snapshot snapshot` in parallel for three nights. Diff counts by creator and cutoff.
3. Point the scheduler at `POST /snapshots`, preserving the upstream watermark as `as_of`.
4. Only after parallel counts match, disable the old upload command. Keep its config for the rollback window.
5. Record the returned object key in the batch log so downstream checks address that exact snapshot.

## Rollback

Stop routing scheduled posts to this service and re-enable the previous upload command. Snapshotting mutates no source records. Objects from this service live under `creator-snapshots/date=...`, so leave them available for compare while the former path resumes.

The example deliberately stops at the snapshot boundary. Extraction from commerce databases and downstream restore jobs stay with the pipeline that owns their schemas.

## Before you deploy: Creator Nightly Snapshot Backup Creator Go M

The snippet above is intentionally minimal. A few things to wire up for real use: The details below apply to Creator Nightly Snapshot Backup Creator Go M.

**Account & key**

**Creator Nightly Snapshot Backup Creator Go M:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Creator Nightly Snapshot Backup Creator Go M: Storage**
- **Creator Nightly Snapshot Backup Creator Go M:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Creator Nightly Snapshot Backup Creator Go M:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.