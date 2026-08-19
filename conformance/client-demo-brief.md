# Margo Conformance Test Suite — Summary

Margo defines how a **device** and a **WFM** (Workload Fleet Manager, the cloud/control-plane side) talk to each other over HTTPS. This suite lets a vendor who built *either side* prove their implementation follows that spec — without needing the other side's real system. We provide a correct mock stand-in, run real HTTP calls against it, and hand back a pass/fail report.

## Two CLIs

- **`conformance.sh`** — Data Generation. Builds test cases (from an OpenAPI spec, a Postman collection, or scenario files) and organizes them into named groups.
- **`run-tests.sh`** — Execution / Runner. Picks a group, fires the actual HTTP calls at a real or mock system, and produces the HTML report.

## Three personas

- **Device Supplier** — vendor brings a real device; we provide a mock WFM to test it against.
- **WFM Supplier** — vendor brings a real WFM; we provide a mock device that fires test calls at it.
- **Application Supplier** — validates that an application package is correctly structured per spec.

## Why it holds up

Tests trace directly to the spec, cover both success and deliberately-broken cases (bad signature, bad cert, etc.), and every report row is a real request/response — expected vs. actual, side by side.
