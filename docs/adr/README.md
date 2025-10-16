# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) for `gh-arc`.

## What is an ADR?

An Architecture Decision Record (ADR) is a document that captures an important architectural decision made along with its context and consequences.

ADRs help us:
- Document why we made specific technical decisions
- Provide context for future maintainers
- Avoid relitigating past decisions
- Onboard new contributors more effectively

## When to Write an ADR

Create an ADR when you make a significant technical decision that:

- Affects the overall architecture or design patterns
- Has long-term implications
- Involves tradeoffs between multiple valid approaches
- Future developers might question or want to understand
- Changes user-facing behavior in meaningful ways

Examples:
- Choosing between different design patterns
- Adding or changing core functionality
- Selecting dependencies or frameworks
- Making security or performance tradeoffs

## ADR Format

Each ADR should follow this structure:

```markdown
# ADR XXXX: Title of Decision

## Status

[Proposed | Accepted | Deprecated | Superseded by ADR-YYYY]

## Date

YYYY-MM-DD

## Context

What is the issue we're addressing? What factors are at play?
Include relevant background information, constraints, and requirements.

## Decision

What decision did we make? Be specific and concrete.
Include implementation details if relevant.

## Alternatives Considered

### Option 1: Name
Description, Pros, Cons, Verdict

### Option 2: Name
Description, Pros, Cons, Verdict

(Include the chosen option here as well)

## Consequences

### Positive
- What becomes easier or better

### Negative
- What becomes harder or worse

### Neutral
- Other effects worth noting

## Implementation References

Links to relevant code, commits, or documentation.

## Related Documentation

External resources or related ADRs.

## Notes

Any additional context or future considerations.
```

## ADR Lifecycle

1. **Proposed**: Decision is under discussion
2. **Accepted**: Decision has been approved and implemented
3. **Deprecated**: Decision is no longer relevant but kept for historical context
4. **Superseded**: Decision has been replaced by a newer ADR

## Naming Convention

ADRs are numbered sequentially with four digits:
- `0001-automatic-force-with-lease-for-rebased-branches.md`
- `0002-next-decision.md`
- etc.

Use descriptive, kebab-case names after the number.

## Modifying ADRs

ADRs should generally not be modified after acceptance, except for:
- Fixing typos or formatting
- Adding implementation references
- Updating status (e.g., Accepted → Deprecated)

If a decision needs to change, create a new ADR that supersedes the old one.

## Resources

- [ADR GitHub Organization](https://adr.github.io/)
- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) (original article by Michael Nygard)
- [ADR Tools](https://github.com/npryce/adr-tools)

## Current ADRs

| Number | Title | Status | Date |
|--------|-------|--------|------|
| [0001](0001-automatic-force-with-lease-for-rebased-branches.md) | Automatic Force-With-Lease for Rebased Branches | Accepted | 2025-10-16 |
