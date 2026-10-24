# Security policy

## Supported versions

The project is pre-alpha. Only the `main` branch receives fixes.

## Reporting a vulnerability

Please do not open a public issue for security problems.

Report vulnerabilities privately through GitHub's
[private vulnerability reporting](https://github.com/wilfriedago/tigerbeetle-platform/security/advisories/new)
or by email to wilfried.ago@bftgroup.co.

Include the affected component (api, console, deploy), steps to reproduce and the impact you expect.
You will receive an acknowledgement within five working days. We will agree on a disclosure date with you
once a fix is available.

## Scope

In scope: the Go API service, the generated clients, the console, the compose stack and the CI workflows
in this repository. Out of scope: TigerBeetle and PostgreSQL themselves, which have their own security
processes, and the infrastructure of anyone who deploys this project.
