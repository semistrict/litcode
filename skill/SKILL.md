---
name: litcode
description: Use this skill when you need to validate that markdown documentation matches source code with the litcode CLI in another repository, especially for doc drift checks, source coverage checks, Mermaid-based structural documentation, automatic fixes, or HTML rendering.
---

# litcode

Use this skill when a repository uses `litcode` to keep markdown documentation aligned with source code.

## When to use

- The user asks you to run `litcode`, `make check`, `litcode fix`, or `litcode html-docs`
- The repository has markdown code fences with `file=` / `lines=` references
- The task is about doc/source drift, missing documentation coverage, or Mermaid documentation diagrams

## Instructions

1. Start with `litcode check --docs ./docs --source .` or the repo’s wrapper target such as `make check`.
2. Treat `FIXABLE:` output as drift or minor edits that `litcode fix` can resolve automatically.
3. Treat `INVALID:` output as a referenced block that no longer matches source. The diff shown is against the closest match in the file, not just a deletion dump.
4. Treat `MISSING:` output as source lines that still need documentation coverage. Lines already reported via an `INVALID` block are suppressed from `MISSING`.
5. If the mismatch is minor drift, try `litcode fix --docs ./docs --source .`.
6. Re-run `litcode check` after any doc or source change.
7. If the user wants rendered docs, run `litcode html-docs --docs ./docs --out ./out/docs` or the repo’s wrapper target.

## Conventions to know

- Verbatim code blocks usually look like ```` ```go file=path/to/file.go lines=10-25 ````.
- If `lines=` is omitted, `litcode` matches the block content against the file.
- You may add brief explanatory comment lines inside a verbatim block when that improves readability; `litcode` strips comment-only explanation lines before matching.
- `litcode` warns when a block starts with comment-only lines, and also when a top-level comment appears later in the block after code has started.
- Large verbatim blocks may omit a contiguous middle section with a comment-only ellipsis marker such as `// ...` or `# ...`.
- Abbreviated blocks do not require `lines=`; `litcode` expands them when the prefix/suffix pair matches a unique source span and fails if the abbreviated block is ambiguous.
- Mermaid can document structure instead of copying source verbatim.
- For Mermaid coverage, include a symbol label such as `title function: ParseFile` or `title method: Handle`.
- Syntax-only lines such as opening or closing braces do not need their own coverage; `litcode` only reports missing non-skippable source lines.
- Test files may be referenced and validated, but they do not require missing-coverage reporting by default.
- `testdata/`, `vendor/`, and `node_modules/` are skipped entirely by default.

## Let documentation drive code structure

Writing the narrative is a design tool, not just a documentation step. If you hit awkwardness while documenting — a function that's hard to explain, a block that needs too many caveats, tangled control flow that resists a clean narrative — treat that as a smell test. Refactor the source code (split functions, rename things, simplify logic) until the explanation flows naturally. The documentation difficulty is surfacing real complexity that readers and maintainers will also struggle with.

## Good defaults

- Prefer fixing the target repository’s docs before changing source references broadly.
- Use Mermaid when explaining control flow or method structure is clearer than repeating the full code.
- Add short explanatory comment lines inside source blocks when they materially help an agent or reader understand the example, but keep them sparse.
- Break long functions into multiple documented segments when that is clearer than forcing one oversized block.
- Prefer splitting a long function into multiple focused code blocks over keeping one large block with many explanatory source comments.
- If `litcode` warns about top-level comments inside a block, split the block at the comment and turn the comment into prose between the resulting blocks.
- Use an ellipsis comment to collapse uninteresting middle lines in long verbatim blocks when the prefix and suffix already communicate the point of the example.
- Do not try to document standalone braces or other syntax-only elements just to satisfy coverage.
- After `litcode html-docs`, inspect the configured output directory, commonly `out/docs/`.
- Do not modify `litcode` itself unless the user explicitly asks for tool changes.

## Documentation structure

- Do NOT number documentation files (e.g. `01-overview.md`). Use descriptive names like `overview.md`, `checker.md`.
- Create an index/overview file (e.g. `overview.md`) that links to all other doc files. This serves as the entry point and table of contents.
- Split large doc files along natural boundaries — one doc file per source module or major concern. A doc file over ~300 lines should be split.
- Each doc file should have a navigation header linking back to the overview and to previous/next pages for linear reading.
