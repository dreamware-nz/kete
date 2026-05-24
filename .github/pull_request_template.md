# File a PR if this is a small, scoped change.
#
# For anything bigger, please open or comment on a brief in
# process/briefs/ first. The chain is idea → brief → ADR(s) →
# plan (phased) → execute. See CRUSH.md / CLAUDE.md for the rules
# and process-driver / process-* skills for the workflow.

## Summary

<!-- One paragraph: what changes and why. Reference any ADR/brief
     numbers that govern this change. -->

## Decision trail

<!-- If this PR makes a non-trivial decision (added a dep, changed
     a public API, picked between two viable approaches), drop a
     `Decision:` git trailer in the commit and link the ADR or
     `process/drift.md` line here. ADR 0006 (byte-exact bodies)
     and ADR 0015 (three-upstream selection) are the most common
     ones to read before touching the proxy. -->

## Test plan

- [ ] `make test` passes
- [ ] `make build` produces a binary that responds to `--version`
- [ ] If touching the proxy/adapters: live-smoked against a real
      upstream (note which model/region/account)
- [ ] If touching docs/cli: `make docs` regenerated

## Honest gaps

<!-- Anything that *would* have warranted an ADR or brief revision
     but you didn't write one because the change was too small:
     name it here so the next process-retrospect can pick it up. -->
