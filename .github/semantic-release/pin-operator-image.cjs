// semantic-release plugin: pin the operator image in the release install.yaml
// to the version being released. The committed manifest keeps :latest, only the
// release asset is pinned, so a pinned install runs that release's controller.
const assert = require("node:assert");
const fs = require("node:fs");

const MANIFEST = "manifests/install.yaml";
const IMAGE = "ghcr.io/inference-gateway/operator";
const IMAGE_RE = new RegExp(`(image: ${IMAGE}):\\S+`, "g");

function pin(manifest, version) {
  const pinned = manifest.replace(IMAGE_RE, `$1:${version}`);
  if (pinned === manifest) {
    throw new Error(`No "${IMAGE}" image found in ${MANIFEST}`);
  }
  return pinned;
}

// version has no leading "v" - goreleaser tags images with {{ .Version }}.
async function prepare(_pluginConfig, { nextRelease, logger }) {
  const manifest = await fs.promises.readFile(MANIFEST, "utf8");
  await fs.promises.writeFile(MANIFEST, pin(manifest, nextRelease.version));
  logger.log(`Pinned ${IMAGE} to ${nextRelease.version} in ${MANIFEST}`);
}

module.exports = { prepare, pin };

if (require.main === module) {
  assert.equal(
    pin("        image: ghcr.io/inference-gateway/operator:latest\n", "0.25.1"),
    "        image: ghcr.io/inference-gateway/operator:0.25.1\n",
  );
  assert.throws(() => pin("image: busybox:latest\n", "0.25.1"));
  assert.equal(
    pin(fs.readFileSync(MANIFEST, "utf8"), "0.25.1").match(
      /ghcr.io\/inference-gateway\/operator:0\.25\.1/g,
    ).length,
    1,
  );
  console.log("ok");
}
