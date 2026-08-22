# Shevanio AI release trust anchor

This document is the maintainer-controlled trust channel for Shevanio AI release signatures. It was published in version-controlled project history before the first signed release.

## Active Minisign public key

Canonical public-key payload:

```text
RWSs0TeOUIpmrWed/2kNsFKiB7Lq+XGSv0Pf7Li5fbYajJDvtKRss3HU
```

SHA256 fingerprint:

```text
102d3ca1651bc36b01402e6effe3cadec392d930f90dd86623cf0b9ca78e3f80
```

The fingerprint is SHA256 of the canonical public-key payload UTF-8 bytes with no trailing newline. Reproduce it with:

```bash
printf '%s' 'RWSs0TeOUIpmrWed/2kNsFKiB7Lq+XGSv0Pf7Li5fbYajJDvtKRss3HU' | sha256sum
```

## Verify a release

Verify the signed checksum manifest before trusting an archive checksum:

```bash
minisign -VQm checksums.txt \
  -x checksums.txt.minisig \
  -P 'RWSs0TeOUIpmrWed/2kNsFKiB7Lq+XGSv0Pf7Li5fbYajJDvtKRss3HU'
```

The trusted comment must identify the exact repository and tag being installed:

```text
repo=Shevanio/shevanio-ai;tag=vMAJOR.MINOR.PATCH
```

Release candidates use `vMAJOR.MINOR.PATCH-rc.N`. A key copied only from release assets does not authenticate those same assets. Key rotations are announced by updating this document before publishing with the new key and follow the bounded overlap process in [release-signing.md](release-signing.md).
