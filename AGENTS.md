# Piledriver: Bug Hunter

Systematic bug hunting using formal methods and verification techniques.

## Philosophy

**Middle-out modeling**: Start with a suspected bug, define a boundary around it, formally verify what's inside, and treat what's outside as explicit assumptions.

This is NOT "spec then implement." This is "reverse engineer, verify, probe."

## Workflow

```
(init) → RECONNAISSANCE ←→ SCOPING ←→ ASSUMPTIONS ←→ VERIFICATION → TESTING → REPORT
              ↓ (skip)        ↑
              └───────────────┘
```

| Phase | Goal |
|-------|------|
| **RECONNAISSANCE** | Automated scanning to identify suspects (optional) |
| **SCOPING** | Define what's inside vs outside the verification boundary |
| **ASSUMPTIONS** | Document assumptions about components outside the boundary |
| **VERIFICATION** | Verify using appropriate technique (TLA+, property testing, fuzzing, etc.) |
| **TESTING** | Red/green: commit a failing test that proves the bug, then commit the fix |
| **REPORT** | Synthesize findings for multiple audiences |

Run `piledriver status` for current state. Run `piledriver set-phase <session> <phase>` to transition—the CLI will print detailed guidance for each phase.

## Critical Rules

1. **Human gates**: Get explicit human approval before advancing past SCOPING or ASSUMPTIONS
2. **Reproducers are truth**: A verification result means nothing until confirmed with a real test
3. **Move flexibly**: Go back to any earlier phase if needed, but announce why
4. **Explicit boundaries**: Every component is IN or OUT, never implicit

## Commands

```bash
piledriver init <session>              # Start session (RECONNAISSANCE)
piledriver init <session> --skip-recon # Start session (SCOPING)
piledriver set-phase <session> <phase> # Transition phases
piledriver technique <session> [type]  # View/set verification technique
piledriver check <session> [--sany]    # Run TLC (if using TLA+)
piledriver bug <session> <bug>         # Create reproducer scaffold
piledriver test <session> [bug]        # Run reproducer validation
piledriver pr <session>                # Generate PR draft
piledriver status [session]            # Show state + guidance
piledriver summary <session> [text]    # View/set task summary
piledriver critique <session> <context> # Get feedback from critic agent
```

Session names must be lowercase.

## Verification Techniques

| Technique | Best For |
|-----------|----------|
| **TLA+** | Concurrent/distributed systems, state machines, protocol bugs |
| **Property testing** | Data transformation, pure functions, invariants |
| **Fuzzing** | Input validation, parsing, crash bugs |
| **Differential testing** | Comparing implementations |
| **Manual review** | Simple logic bugs |

Set with `piledriver technique <session> <type>`.

## Suggested Tools

Quick reference for language-specific tooling. These are suggestions, not requirements.

### Go
- Race detection: `go test -race ./...`
- Static analysis: `staticcheck`, `go vet`, `golangci-lint`
- Property testing: `rapid`
- Fuzzing: `go test -fuzz=FuzzName`

### Python
- Static analysis: `ruff`, `mypy`
- Property testing: `hypothesis`
- Fuzzing: `atheris`

### Rust
- Race detection: Built into `cargo test` with `--release`
- Static analysis: `clippy`
- Property testing: `proptest`, `quickcheck`
- Fuzzing: `cargo-fuzz`

## Reproducer Contract

Each reproducer has two files:
- `scenario.yaml` - Config (base_commit, fix_commit, setup_commands)
- `reproduce.sh` - Simple test script (exit 0 = pass, exit 1 = fail)

Piledriver handles git operations. You just write the test and fill in commit SHAs.

Requirements:
1. Test ONE specific bug
2. Keep reproduce.sh simple - just the test command
3. Base commit: test should FAIL (bug present)
4. Fix commit: test should PASS (bug fixed)
5. Test must be deterministic

## Testing Phase: Red/Green Workflow

After verification identifies a bug, prove it's real with two commits:

**Commit 1 — Red (test only):**
1. `piledriver bug <session> <bug-name>` to scaffold the reproducer
2. Write `reproduce.sh` — a test that exposes the bug
3. Fill in `base_commit` in `scenario.yaml` (current HEAD)
4. Commit ONLY the test. Do not include any fix.
5. `piledriver test <session>` — confirm the test fails (bug confirmed)

**Check in with the human before proceeding to the fix.**

**Commit 2 — Green (fix only):**
1. Write the minimal fix for the bug
2. Commit the fix
3. Update `fix_commit` in `scenario.yaml` to this new commit
4. `piledriver test <session>` — confirm test now passes

**Do not modify `reproduce.sh` between red and green.** The same test must fail before the fix and pass after. If you believe `reproduce.sh` must change, stop and explain to the human why this is still a fair red/green test and not gaming the results. Only proceed with their approval.

## Report Structure

Reports in REPORT phase should explain findings to multiple audiences:

**Multi-Audience Sections**:
- **Non-technical decision-makers**: Why does this matter for the business/product?
- **Technical leadership**: Where does this stand in the pantheon of tech problems?
- **Users of this software**: Where might this appear in real-world use?
- **Engineer fixing this**: Detailed technical breakdown and fix guidance

**Multi-Level Explanations** (include at least 3 levels):
1. Layperson summary (1-2 sentences anyone can understand)
2. Technical overview (for engineers unfamiliar with codebase)
3. Expert deep-dive (for engineers who will implement the fix)

## Critic Agent

Use `piledriver critique <session> "<context>"` to get feedback from a critic agent (default: Gemini).

The critic helps:
- Decide when to advance to the next phase
- Identify gaps in analysis
- Maintain quality bar
- Validate that null results are OK (not every investigation finds bugs)

Configure the critic command in `.piledriver/config.yaml`:
```yaml
critic_command: "gemini -p"  # or "gpt -p", "claude -p", etc.
```
