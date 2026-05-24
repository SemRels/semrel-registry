# Governance

## Overview

semrel-registry is an open source project governed by its maintainers and community contributors. This repository defines the SemRels plugin registry, schemas, and related automation.

## Scope

This governance applies to the full repository, including schemas, generated assets, workflows, documentation, and any future `api/` subtree unless a more specific local override is added.

## Project Roles

### Contributor
Anyone who opens an issue, submits a PR, improves documentation or participates in discussions. No formal process required — just follow the contribution guidelines and the [Code of Conduct](CODE_OF_CONDUCT.md).

### Reviewer
An experienced contributor who has demonstrated good judgement and is trusted to review PRs in a specific area. Reviewers can approve PRs but cannot merge without a Maintainer approval.

### Maintainer
Maintainers have merge rights and are responsible for the overall project health. The current list of maintainers is in [MAINTAINERS.md](MAINTAINERS.md).

**To become a Maintainer:**
1. Have a track record of quality contributions (code, docs, community) over at least 3 months
2. Be nominated by an existing Maintainer
3. Receive approval from 2/3 of current Maintainers (lazy consensus over 7 days)
4. Be added to MAINTAINERS.md and CODEOWNERS via a PR

**Stepping down / Emeritus:**
Maintainers who are no longer active should move to Emeritus status by opening a PR to update MAINTAINERS.md.

## Decision Making

We use **lazy consensus**: a proposed change is accepted unless a Maintainer explicitly objects within 7 days. For significant architectural decisions, open a GitHub Discussion or Issue first.

**Voting** (if lazy consensus fails): simple majority of active Maintainers. Each Maintainer has one vote. Votes are recorded in the relevant GitHub Issue or Discussion.

## Changes to Governance

Changes to this document require a PR approved by 2/3 of current Maintainers.
