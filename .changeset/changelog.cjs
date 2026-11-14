/**
 * Custom Changeset Changelog Generator
 * Formats bullet points cleanly without prepending git commit SHAs.
 */

const getReleaseLine = async (changeset) => {
  const lines = changeset.summary
    .split("\n")
    .map((l) => l.trimEnd())
    .filter((l) => l.length > 0);

  const formatted = lines.map((l, idx) => {
    if (idx === 0) {
      return l.startsWith("- ") ? l : `- ${l}`;
    }
    return `  ${l}`;
  });

  return formatted.join("\n");
};

const getDependencyReleaseLine = async () => "";

module.exports = {
  getReleaseLine,
  getDependencyReleaseLine,
  default: {
    getReleaseLine,
    getDependencyReleaseLine,
  },
};
