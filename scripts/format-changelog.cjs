#!/usr/bin/env node

const fs = require("fs");
const path = require("path");

const changelogPath = path.resolve(__dirname, "../CHANGELOG.md");
if (!fs.existsSync(changelogPath)) {
  process.exit(0);
}

let content = fs.readFileSync(changelogPath, "utf-8");

const today = new Date().toISOString().split("T")[0];

// 1. Transform Changeset's raw "## X.Y.Z" into "### [X.Y.Z] - YYYY-MM-DD\n\n---"
content = content.replace(/^## (\d+\.\d+\.\d+.*)$/gm, (match, version) => {
  return `### [${version.trim()}] - ${today}\n\n---`;
});

// 2. Fix unbracketed headers "### X.Y.Z - YYYY-MM-DD" -> "### [X.Y.Z] - YYYY-MM-DD"
content = content.replace(
  /^### (?![#\[])(\d+\.\d+\.\d+.*?)\s*-\s*(\d{4}-\d{2}-\d{2})/gm,
  (match, version, date) => {
    return `### [${version.trim()}] - ${date}\n\n---`;
  },
);

// 3. Map Changeset section headers to repository conventions:
//    "### Minor Changes" / "### Major Changes" -> "#### Added"
//    "### Patch Changes" -> "#### Updated"
content = content.replace(/^### (?:Minor|Major) Changes$/gm, "#### Added");
content = content.replace(/^### Patch Changes$/gm, "#### Updated");

// 4. Strip commit hashes like "- 753e591: " -> "- "
content = content.replace(/^- [a-f0-9]{7,8}:\s*/gm, "- ");

// 5. Ensure proper blank line spacing around "---"
content = content.replace(/([^\n])\n---/g, "$1\n\n---");
content = content.replace(/---\n([^\n])/g, "---\n\n$1");

// 6. Ensure proper blank line after "#### Added" or "#### Updated"
content = content.replace(/^(#### (?:Added|Updated))\n(?!\n)/gm, "$1\n\n");

// 7. Ensure consecutive duplicate "---" are removed
content = content.replace(/\n---\s*\n\s*---\n/g, "\n---\n");

// 8. Clean up triple or more consecutive newlines to double newlines
content = content.replace(/\n{3,}/g, "\n\n");

fs.writeFileSync(changelogPath, content, "utf-8");
console.log(
  "CHANGELOG.md formatted successfully to match repository standard.",
);
