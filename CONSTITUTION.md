# Messaging — Constitution

> **Status:** Active. This document is the project's authoritative
> rule set. When a rule here conflicts with `CLAUDE.md`, `AGENTS.md`,
> or any guide, the Constitution wins.

## Mission

See README.md.

## Mandatory Standards

1. **Reproducibility:** every change is reproducible from a clean
   clone (`git clone <repo> && <project bootstrap>`); no hidden steps.
2. **Tests track behavior, not code:** test what the user-visible
   behavior is, not what the implementation looks like.
3. **No silent skips, no silent mocks above unit tests.**
4. **Conventional Commits** for all commits.
5. **SSH-only for git operations** (`git@…`); HTTPS prohibited.

## Numbered Rules

<!-- Rules are numbered CONST-NNN. New rules append. Removed rules
     keep their number with a "**Retired:** …" line. -->

<!-- BEGIN host-power-management addendum (CONST-033) -->

### CONST-033 — Host Power Management is Forbidden

**Status:** Mandatory. Non-negotiable. Applies to every project,
submodule, container entry point, build script, test, challenge, and
systemd unit shipped from this repository.

**Rule:** No code in this repository may invoke a host-level power-
state transition (suspend, hibernate, hybrid-sleep, suspend-then-
hibernate, poweroff, halt, reboot, kexec) on the host machine. This
includes — but is not limited to:

- `systemctl {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot,kexec}`
- `loginctl {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot}`
- `pm-{suspend,hibernate,suspend-hybrid}`
- `shutdown {-h,-r,-P,-H,now,--halt,--poweroff,--reboot}`
- DBus calls to `org.freedesktop.login1.Manager.{Suspend,Hibernate,HybridSleep,SuspendThenHibernate,PowerOff,Reboot}`
- DBus calls to `org.freedesktop.UPower.{Suspend,Hibernate,HybridSleep}`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to any value other than `'nothing'` or `'blank'`

**Why:** The host runs mission-critical parallel CLI-agent and
container workloads. On 2026-04-26 18:23:43 the host was auto-
suspended by the GDM greeter's idle policy mid-session, killing
HelixAgent and 41 dependent services. Recurring memory-pressure
SIGKILLs of `user@1000.service` (perceived as "logged out") have the
same outcome. Auto-suspend, hibernate, and any power-state transition
are unsafe for this host.

**Defence in depth (mandatory artifacts in every project):**
1. `scripts/host-power-management/install-host-suspend-guard.sh` —
   privileged installer, manual prereq, run once per host with sudo.
   Masks `sleep.target`, `suspend.target`, `hibernate.target`,
   `hybrid-sleep.target`; writes `AllowSuspend=no` drop-in; sets
   logind `IdleAction=ignore` and `HandleLidSwitch=ignore`.
2. `scripts/host-power-management/user_session_no_suspend_bootstrap.sh` —
   per-user, no-sudo defensive layer. Idempotent. Safe to source from
   `start.sh` / `setup.sh` / `bootstrap.sh`.
3. `scripts/host-power-management/check-no-suspend-calls.sh` —
   static scanner. Exits non-zero on any forbidden invocation.
4. `challenges/scripts/host_no_auto_suspend_challenge.sh` — asserts
   the running host's state matches layer-1 masking.
5. `challenges/scripts/no_suspend_calls_challenge.sh` — wraps the
   scanner as a challenge that runs in CI / `run_all_challenges.sh`.

**Enforcement:** Every project's CI / `run_all_challenges.sh`
equivalent MUST run both challenges (host state + source tree). A
violation in either channel blocks merge. Adding files to the
scanner's `EXCLUDE_PATHS` requires an explicit justification comment
identifying the non-host context.

**See also:** `docs/HOST_POWER_MANAGEMENT.md` for full background and
runbook.

<!-- END host-power-management addendum (CONST-033) -->

## Definition of Done

A change is done when:

1. The code change is committed.
2. All project-level tests pass on a clean clone.
3. All challenges in `challenges/scripts/` pass on the running host.
4. Governance docs (`CONSTITUTION.md`, `AGENTS.md`, `CLAUDE.md`) are
   coherent with the change.

## See also

- `README.md` — project overview, quickstart.
- `AGENTS.md` — guidance for AI coding agents (Codex, Cursor, etc.).
- `CLAUDE.md` — guidance specifically for Claude Code.
- `docs/HOST_POWER_MANAGEMENT.md` — CONST-033 background and runbook.

<!-- BEGIN CONST-035 explicit anchor (cascaded 2026-04-29) -->

## CONST-035 — Anti-Bluff Tests & Challenges (explicit anchor)

This rule is identified as **CONST-035** in the umbrella project's
governance index. The rule is already enforced in this submodule via
the Article XI / Anti-Bluff sections above. This anchor adds the
explicit identifier so automated audits (`grep -r CONST-035 .`) can
locate the rule by its canonical name.

**Operative rule:** A test or Challenge that PASSES is a CLAIM that
the tested behavior **works for the end user of the product**. Every
PASS MUST guarantee:

a. **Quality** — the feature behaves correctly under the inputs an
   end user will send, including malformed input, edge cases, and
   concurrency that real workloads produce.
b. **Completion** — the feature is wired end-to-end from public API
   surface down to backing infrastructure, with no stub / placeholder
   / "wired lazily later" gaps that silently 503.
c. **Full usability** — a CLI agent / SDK consumer / direct client
   following the documented interface SUCCEEDS without having to know
   internal aliases, hidden flags, or undocumented dispatch quirks.

A passing test that doesn't certify all three is a **bluff** and MUST
be tightened, or marked `t.Skip("...SKIP-OK: #<ticket>")` so absence
of coverage is loud rather than silent.

**Bluff taxonomy** (each pattern observed in HelixAgent and now
forbidden):

- **Wrapper bluff** — assertions PASS but the wrapper's exit-code
  logic is buggy. Use `! grep -qs "|FAILED|" "$LOG"` style counters,
  never inline arithmetic on a command that prints AND exits non-zero.
- **Contract bluff** — the system advertises a capability but rejects
  it in dispatch. Every advertised capability MUST be exercised by a
  test or Challenge that actually invokes it.
- **Structural bluff** — `check_file_exists "foo_test.go"` passes if
  the file is present but doesn't run the test. File-existence checks
  MUST be paired with at least one functional assertion.
- **Comment bluff** — a code comment promises a behavior the code
  doesn't actually have. Documentation MUST be re-verified against
  the code on every change touching the documented function.
- **Skip bluff** — `t.Skip("not running yet")` without a `SKIP-OK:
  #<ticket>` marker silently passes. Every skip needs the marker; CI
  fails on bare skips.

**Verification of CONST-035 itself:** deliberately break the feature
(e.g. `kill <service>`, swap a password, unset an env var) and re-run
the test/Challenge. It MUST FAIL. If it still passes, the test is
non-conformant and MUST be tightened.

**Cross-reference:** umbrella `CONSTITUTION.md` Article XI §§ 11.1 –
11.9; root `CLAUDE.md` "Mandatory Development Standards" rule 27.

<!-- END CONST-035 explicit anchor (cascaded 2026-04-29) -->
