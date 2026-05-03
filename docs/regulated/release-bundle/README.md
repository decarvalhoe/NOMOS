# Release Evidence Bundle

This folder is reserved for release-level evidence.

The release bundle must answer whether a release is fit for the claim being made.

## Minimum Contents

- release identity, commit, tag, workflow run, and build provenance;
- intended-use scope;
- control matrix version;
- validation pack version;
- generated report hashes;
- source/corpus hashes;
- open deviations;
- waivers with expiry;
- approval status;
- public claim boundary;
- Praxis evidence status.

Use `templates/regulated/release-evidence-bundle.yaml` as the starting structure.

## v0.1.0-ALPHA Bundle Expectation

The alpha release bundle must include or reference:

- release tag `v0.1.0-ALPHA`;
- release commit;
- PR and CI evidence;
- local verification commands and results;
- RBOK POC validation dossier;
- strict fidelity proof status;
- public claim boundary;
- open issue list for known regulated gaps;
- statement that the release is a GitHub pre-release and not a regulated certification.

Missing customer validation, approval, training, or licensed-reference evidence must remain visible as gaps or future work.
