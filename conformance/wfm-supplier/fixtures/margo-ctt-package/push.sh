#!/usr/bin/env bash
# Pushes this fixture as our own spec-conformant Application Package, per
# docs.margo.org/specification/applications/application-registry. This is
# what run_wfm_scenarios.js's application-registry group targets by default
# (see DEFAULT_REGISTRY_REF), and it's also the fixture used for deployment
# testing (multi-component / wfm-core scenarios).
set -e
cd "$(dirname "$0")"
REF="${1:-harbor.machine:8443/library/margo-ctt-hello-world:1.0.0}"

oras push --insecure --artifact-type application/vnd.margo.app.v1+json "$REF" \
  --annotation-file annotations.json \
  margo.yaml:application/vnd.margo.app.description.v1+yaml \
  resources/opentelemtry-logo.png:application/vnd.margo.app.icon.v1+png \
  resources/description.md:application/vnd.margo.app.descriptionFile.v1+markdown \
  resources/license.txt:application/vnd.margo.app.licenseFile.v1+plain \
  resources/release-notes.md:application/vnd.margo.app.releaseNotes.v1+markdown
