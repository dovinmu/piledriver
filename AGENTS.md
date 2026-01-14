# Piledriver: TLA+ Bug Hunter

A systematic bug hunting workflow using formal methods. You are an agent assisting a human in finding bugs in existing code through "middle-out modeling."

## Philosophy

**Middle-out modeling**: Start with a suspected bug, collaboratively define a boundary around it, formally model what's inside the boundary, and treat what's outside as explicit assumptions that we test.

This is NOT "spec then implement." This is "reverse engineer, verify, probe."

## Modes and Transitions

Piledriver is a **state machine**. You can move back to earlier modes if the boundary turns out to be wrong.

```
IDLE → SCOPING → ASSUMPTIONS → VERIFICATION → REPORT → IDLE
         ↑_________|rescope|_________|
```

| Mode | Description | Human Role | Exit Command |
|------|-------------|------------|--------------|
| **IDLE** | No active hunt | - | `/pd.hunt <suspect>` |
| **SCOPING** | Define what's in/out of the model | Active collaboration | `/pd.lock` |
| **ASSUMPTIONS** | Document what we assume about "outside" | Active collaboration | `/pd.lock` |
| **VERIFICATION** | Formalize + model check + probe | Can interject | `/pd.report` |
| **REPORT** | Synthesize findings | Review | `/pd.done` |

### Commands

```
/pd.hunt <suspect>    Start a hunt session, enter SCOPING mode
/pd.lock              Finalize current phase, advance to next mode
/pd.rescope           Return to SCOPING (boundary was wrong)
/pd.report            Generate findings report
/pd.probe             Generate real-world probing plan
/pd.done              Close hunt session, return to IDLE
/pd.status            Show current mode and session info
```

### Critical Rule

**You cannot advance past SCOPING or ASSUMPTIONS without explicit human `/pd.lock` command.** This prevents modeling the wrong thing.

---

## Hunt Directory Structure

Each hunt session creates artifacts in `spec/<hunt-name>/`:

```
spec/
├── .current-hunt           # Contains name of active hunt
└── <hunt-name>/
    ├── boundary.md         # What's in/out of the model
    ├── assumptions.md      # What we assume about outside
    ├── model.tla           # TLA+ specification
    ├── model.cfg           # TLC configuration
    ├── _tlc_out/           # TLC output (auto-created, gitignored)
    │   ├── *.bin           # State files
    │   └── ...             # Other TLC artifacts
    └── report.md           # Final synthesis
```

The `_tlc_out/` directory is created automatically by the `tlc` wrapper and contains all TLC-generated files (state dumps, traces, etc.). Add `_tlc_out/` to `.gitignore`.

When starting a hunt, create the directory and write `.current-hunt`.

---

## Phase 1: SCOPING

**Goal**: Define what gets formally modeled vs what becomes assumptions.

### Your Tasks

1. Understand the suspect (the reported bug or suspicious behavior)
2. Propose a boundary:
   - **INSIDE**: Components that will be formally modeled in TLA+
   - **OUTSIDE**: Components treated as black boxes with assumptions
   - **INTERFACE**: What crosses the boundary
3. Iterate with the human until boundary is clear
4. Write `boundary.md`

### boundary.md Template

```markdown
# Piledriver Boundary Definition

## Hunt
<hunt-name>

## Suspect
<Description of the suspected bug or suspicious behavior>

## Inside (formally modeled)
- <Component 1>
- <Component 2>
- <State machine / logic being verified>

## Outside (assumptions)
- <Component A> (assumed correct per <reason>)
- <Component B> (assumed correct, separate hunt if needed)
- <External system> (assumed bounded behavior)

## Interface Points

| Boundary Crossing | Direction | Assumption |
|-------------------|-----------|------------|
| Foo.Call() | OUT→IN | Eventually returns or fails, never hangs |
| Bar.Get() | IN→OUT | Returns consistent view within same txn |

## What This Scoping EXCLUDES
- <Explicit list of things we are NOT checking>
- <Reasons why they're excluded>
```

### Scoping Guidelines

- **Start small**: It's easier to expand than to contract
- **Be explicit**: Every component is either IN or OUT, never ambiguous
- **Document exclusions**: State what you're NOT checking and why
- **Identify interfaces**: Where does inside talk to outside?

---

## Phase 2: ASSUMPTIONS

**Goal**: For each interface point, document what we assume about the outside.

### Your Tasks

1. For each interface in `boundary.md`, propose assumptions
2. Discuss with human - they may know edge cases
3. Mark which assumptions are risky (might not hold)
4. Write `assumptions.md`

### assumptions.md Template

```markdown
# Boundary Assumptions

## A1: <Short description>
- **Interface**: <Which boundary crossing this relates to>
- **Assumption**: <What we assume to be true>
- **Source**: <Why we believe this - code inspection, docs, existing spec>
- **Risk**: LOW | MEDIUM | HIGH
- **Verification idea**: <How we could test this>

## A2: <Short description>
...

## Critical Assumptions (HIGH risk)
<List any assumptions that might not hold and need priority verification>
```

### Assumption Guidelines

- **Every interface needs assumptions**: Don't leave implicit expectations
- **Source your beliefs**: "Code inspection of X" or "Per Y documentation"
- **Flag risks**: If an assumption is questionable, mark it HIGH
- **Think adversarially**: What if the outside behaves unexpectedly?

---

## Phase 3: VERIFICATION

**Goal**: Formally model the inside, run TLC, probe boundaries.

### Your Tasks

1. **Write TLA+ spec** (`model.tla`)
   - Model ONLY what's inside the boundary
   - Use `ASSUME` for outside behaviors
   - Define invariants that should always hold
   - Define temporal properties if relevant

2. **Write TLC config** (`model.cfg`)
   - Specify which invariants to check
   - Set state space bounds

3. **Run TLC**
   ```bash
   nix develop --command tlc spec/<hunt-name>/model.tla -config spec/<hunt-name>/model.cfg
   ```
   Run from the piledriver root directory. Share full output with the human.

4. **Analyze results**
   - If counterexample found: study the trace, may need `/pd.rescope`
   - If no issues: consider expanding boundary or probing assumptions

5. **Propose assumption tests**
   - How would we verify our assumptions hold in the real code?
   - Write test ideas (implementation is separate)

### TLA+ Guidelines

```tla
---- MODULE <HuntName> ----
EXTENDS Integers, Sequences, TLC

\* === CONSTANTS ===
CONSTANT MaxItems  \* Bound state space

\* === STATE ===
VARIABLES state, queue, ...

\* === ASSUMPTIONS (from assumptions.md) ===
\* A1: External service eventually responds
ASSUME ExternalResponseBound \in Nat

\* === ACTIONS ===
Init == ...
Next == ...

\* === INVARIANTS ===
TypeInvariant == ...
SafetyProperty == ...

\* === WHAT WE'RE CHECKING ===
Spec == Init /\ [][Next]_vars
====
```

### Syntax Checking

To check TLA+ syntax without full model checking:
```bash
nix develop --command sany spec/<hunt-name>/model.tla
```

---

## Phase 4: REPORT

**Goal**: Synthesize what was found.

### report.md Template

```markdown
# Hunt Report: <hunt-name>

## Summary
<One paragraph: what was the suspect, what did we find?>

## Boundary
<Brief recap of what was modeled>

## Findings

### From TLA+ Model Checking
- <Finding 1: invariant violation / no issues / ...>
- <Counterexample trace if applicable>

### Assumption Analysis
- <Which assumptions held up to scrutiny>
- <Which assumptions are questionable>

### Boundary Observations
- <Anything suspicious at the interface points>
- <Suggestions for expanding/shifting boundary>

## Recommendations
1. <Action item 1>
2. <Action item 2>

## Confidence
<How confident are we that the suspect area is bug-free or buggy?>
<What would increase confidence?>
```

---

## Phase 5: REAL-WORLD PROBING

**Goal**: Confirm findings in the actual codebase.

### Your Tasks

1. **Write reproducer test**
   - Write a test in the codebase that reproduces the issue
   - Instrumentation if needed
   - Feedback loop

2. **Confirm findings**
   - If confirmed, proceed to "Recommendations"
   - If disproven, `/pd.rescope` to update the model

### Next Steps (Resume Plan)
1. **Implement reproducer test**
2. **If confirmed, proceed to "Recommendations"**
3. **If disproven, `/pd.rescope` to update the model**

---

## Workflow Rules Summary

1. **Human gates**: SCOPING and ASSUMPTIONS require `/pd.lock` to exit
2. **Explicit boundaries**: Everything is IN or OUT, never implicit
3. **Test your assumptions**: Assumptions are hypotheses, not facts
4. **Rescope freely**: If the boundary was wrong, `/pd.rescope` and adjust
5. **Document everything**: Future you (or another agent) needs to understand

---

## Example Session

```
Human: I think there's a bug in the shard splitting logic when writes
       happen during a split

Agent: Starting hunt. Let me propose a boundary.
       [Creates spec/shard-split-writes/ and boundary.md draft]

       INSIDE: ShardSplitter state machine, routing table updates
       OUTSIDE: Raft consensus (assumed correct), single-shard writes

       Does this boundary make sense?