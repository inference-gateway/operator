// semantic-release plugin: pin the operator image to the version being released,
// both in the kustomize overlay that generates manifests/install.yaml and in the
// generated manifest itself. @semantic-release/git commits both, so the release
// tag carries a version-pinned manifest (ArgoCD `path: manifests`) while
// `task manifests` still reproduces exactly what is committed.
const assert = require("node:assert");
const fs = require("node:fs");

const IMAGE = "ghcr.io/inference-gateway/operator";
const ESCAPED = IMAGE.replace(/\./g, "\\.");

const TARGETS = [
  [
    "config/environments/prod/kustomization.yaml",
    new RegExp(`(name: ${ESCAPED}\\n\\s+newTag: )\\S+`),
  ],
  ["manifests/install.yaml", new RegExp(`(image: ${ESCAPED}:)\\S+`, "g")],
];

function pin(content, re, version) {
  if (!content.match(re)) {
    throw new Error(`No "${IMAGE}" reference matching ${re} found`);
  }
  return content.replace(re, `$1${version}`);
}

// version has no leading "v" - goreleaser tags images with {{ .Version }}.
async function prepare(_pluginConfig, { nextRelease, logger }) {
  for (const [file, re] of TARGETS) {
    const content = await fs.promises.readFile(file, "utf8");
    await fs.promises.writeFile(file, pin(content, re, nextRelease.version));
    logger.log(`Pinned ${IMAGE} to ${nextRelease.version} in ${file}`);
  }
}

module.exports = { prepare, pin };

if (require.main === module) {
  for (const [file, re] of TARGETS) {
    const pinned = pin(fs.readFileSync(file, "utf8"), re, "0.25.1");
    assert.equal(pinned.match(/0\.25\.1/g).length, 1, file);
    assert.throws(() => pin("image: busybox:latest\n", re, "0.25.1"));
  }
  console.log("ok");
}
