-- review.lua
-- Demonstrates multi-tasking: independent review tasks run concurrently.
-- Each task is scoped and named. The agent reasons inside each one separately.

local review_scope = scope("review-scope", {
    include      = { "**/*.go", "**/*.lua", "README.md" },
    exclude      = { "vendor/**", ".splash/**" },
    capabilities = { "filesystem.read", "process.execute" },
})

workflow("code-review", {
    scope = review_scope,

    steps = {
        -- Read the source first.
        step("read-code", execute("filesystem.read", {
            path = "main.go",
        })),

        -- Spawn concurrent review tasks.
        step("security-review", task(
            "security-review",
            "Review the code for security vulnerabilities, unsafe patterns, " ..
            "and potential attack surfaces. Be specific."
        )),

        step("performance-review", task(
            "performance-review",
            "Review the code for performance issues, unnecessary allocations, " ..
            "blocking operations, and optimization opportunities."
        )),

        step("style-review", task(
            "style-review",
            "Review the code for Go idioms, naming conventions, " ..
            "error handling patterns, and documentation quality."
        )),

        -- Final synthesis.
        step("synthesize", reason(
            "Synthesize all review findings into a concise report. " ..
            "Prioritize issues by severity: critical, major, minor."
        )),
    }
})
