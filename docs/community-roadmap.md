# Community Roadmap

Where to find approved work and how to claim it without relying on a copied backlog.

## Start here

**[→ Every open issue approved for implementation](https://github.com/Shevanio/shevanio-ai/issues?q=is%3Aissue+is%3Aopen+label%3Astatus%3Aapproved)**

That link is the roadmap. It is a live query, not a list maintained by hand, so
it can never go stale the way a copied table does.

## What `status:approved` guarantees

An issue carrying this label has passed the one gate required before a pull request can be opened:

1. **Implementation is authorized.** CI accepts a closing issue reference only
   when the linked issue carries `status:approved`.
2. **The approved scope is the boundary.** Read the issue body and maintainer
   comments before changing code.

Approval does **not** mean an issue is unclaimed. Check its assignee and recent
comments, then announce that you are taking it. If the label is absent, discuss
the issue instead of opening a pull request.

## Picking one up

1. Confirm the issue is open, approved, and not assigned or claimed in comments.
2. Comment that you are taking it.
3. Read the complete approved scope and verify the current source before editing.
4. Open a PR that closes the issue. CI checks the reference and approval label.

## Reading the labels

| Label | Meaning |
| --- | --- |
| `status:approved` | A PR may be opened. Required by CI. |
| `good first issue` | A smaller entry point for a new contributor. |
| `help wanted` | The maintainers especially welcome outside help. |
| `type:bug` / `type:feature` / `type:docs` / … | The pull-request change category. Exactly one `type:*` label is required. |
| `bug` / `enhancement` / `documentation` | General issue classification. |

Labels describe policy and issue shape, not ownership. The live issue and its
comments remain authoritative.

## Windows contributions

Native Windows coverage is active and green. Pull requests run the curated
`Windows Runtime` release blockers, while the ten-shard `Windows Full Suite`
runs after merges to `main`, every day, and on manual dispatch. If you report a
Windows-only defect, include the failing command, runner architecture, and a
minimal reproduction. See [CI Operations](ci.md) for the current lane split.

## If you want something smaller

Browse [`good first issue`](https://github.com/Shevanio/shevanio-ai/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
or [approved documentation work](https://github.com/Shevanio/shevanio-ai/issues?q=is%3Aissue+is%3Aopen+label%3Astatus%3Aapproved+label%3Atype%3Adocs).
Always confirm the issue is still unclaimed before starting.

## Where the design lives

Before changing review, delivery or SDD behaviour, read:

- [Organic RDD architecture](architecture/organic-rdd.md) — how a candidate
  becomes a receipt, and what each gate validates
- [Review authority threat model](review-authority-threat-model.md) — what the
  authority store defends against, and what it deliberately does not
- [Organic implementation routing](trigger-rules.md) — how work is routed
  before review ever runs

RDD is **Receipt-Driven Development**. The receipt is what every delivery gate
validates; reviewing is one step on the way to producing it.
