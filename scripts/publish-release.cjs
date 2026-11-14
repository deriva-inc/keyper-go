#!/usr/bin/env node

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");

function run(cmd) {
  try {
    return execSync(cmd, {
      encoding: "utf-8",
      stdio: ["ignore", "pipe", "pipe"],
    }).trim();
  } catch (err) {
    return "";
  }
}

const pkgPath = path.resolve(__dirname, "../package.json");
if (!fs.existsSync(pkgPath)) {
  console.error("package.json not found!");
  process.exit(1);
}

const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf-8"));
const version = pkg.version;
const tagName = `v${version}`;

console.log(`Processing release tag ${tagName}...`);

// Create annotated tag if it doesn't already exist locally
const existingLocal = run(`git tag -l "${tagName}"`);
if (!existingLocal) {
  console.log(`Creating local git tag ${tagName}...`);
  try {
    execSync(`git tag -a "${tagName}" -m "Release ${tagName}"`, {
      stdio: "inherit",
    });
  } catch (err) {
    console.error(`Failed to create tag ${tagName}:`, err.message);
    process.exit(1);
  }
} else {
  console.log(`Local git tag ${tagName} already exists.`);
}

// Push tag to origin
console.log(`Pushing tag ${tagName} to GitHub...`);
try {
  execSync(`git push origin "${tagName}"`, { stdio: "inherit" });
  console.log(`Successfully pushed ${tagName} to GitHub!`);
} catch (err) {
  console.log(`Tag ${tagName} might already exist on remote:`, err.message);
}

// Changeset expects stdout in this format to record published packages:
console.log(`New tag:\tv${version}`);
console.log(`New tag:\t${pkg.name}@${version}`);

if (process.env.GITHUB_OUTPUT) {
  fs.appendFileSync(
    process.env.GITHUB_OUTPUT,
    `published=true\nversion=${version}\ntag=${tagName}\n`,
  );
}
