# textlexer documentation

This directory is the documentation home for `textlexer`.

- [specs/core-contract.md](specs/core-contract.md) is the **canonical prose
  specification**: the project purpose, the lexer/consumer boundary,
  terminology, the consumer contract, the execution contract, the intended
  construction capabilities, and the open design questions. Every substantive
  statement in it carries an explicit status — **current contract**, **intended
  capability**, or **open design question** — so the engine available today
  never reads as if it includes capabilities that are still to be designed.
- The [root README](../README.md) is the project README: what the toolkit is,
  what it can do today, a runnable example, installation, and layering. It links
  here for the specification.

## How to read the status labels

The core contract uses a three-level promise ladder:

| Status | Meaning |
| --- | --- |
| **Current contract** | Exists in code at `master`, exercised by committed tests. Consumers may rely on it. A named test or source location is cited. |
| **Intended capability** | Owner-intended, recorded at the semantic level so later work is decidable. **Not** available API; no signature, data structure, or construction order is approved by the statement. |
| **Open design question** | Genuinely undecided. The document names the question and the constraint it must satisfy; it does not choose an answer and does not pretend an answer has owner approval. |

A substantive promise with no status is a documentation bug — flag it.

## Where things live

- **Specification.** [specs/core-contract.md](specs/core-contract.md).
- **Runnable example.**
  [`internal/examples/hello/main.go`](../internal/examples/hello/main.go) — a
  complete program using only the current API, with the rule helpers defined
  alongside.
- **Tests that pin the contract.** The test suite under the repository root
  (`textlexer_test.go`, `textlexer_cursor_test.go`,
  `textlexer_context_test.go`) is the executable evidence the specification
  cites by test name.
- **Upstream cursor.** Text-stream mechanics (reading, buffering, positions,
  checkpoints, replay) come from
  [`github.com/xiam/textreader`](https://github.com/xiam/textreader); the lexer
  owns no text-stream mechanics.

Keep the README and the core contract as paired documentation: when one changes,
review the other in the same change for consistency.
