# How-to: Verify the image before installing

Problem: before you install Hallido on your NAS you want to be sure that
`compose.yaml` and the container image are exactly what our release workflow
built from the public source code — not modified on the way.

You do not have to do this to use Hallido; the NAS guide works without it.
It takes about five minutes on a computer with a terminal (macOS, Linux or
Windows with WSL).

## What is signed

Every release ([ADR 0021](../adr/0021-container-image-supply-chain.md)) is
built and signed by the GitHub workflow `.github/workflows/release.yml` of
`ohm-meter/hallido` — **keyless** with [Sigstore](https://www.sigstore.dev/):
there is no private key that could leak. Instead, the signature certificate
contains the identity of that workflow and the release tag, and every
signature is recorded in the public transparency log (Rekor).

| File in the release | What it contains |
|---|---|
| `compose.yaml` | The project file for the NAS, with the image **pinned by digest** (`image: ghcr.io/ohm-meter/hallido@sha256:…`) |
| `compose.yaml.sha256` | SHA-256 checksum of `compose.yaml` |
| `compose.yaml.sigstore.json` | Signature of `compose.yaml` |
| `IMAGE_DIGEST.txt` | The signed image reference |
| `hallido-image.spdx.json` | Software bill of materials (SBOM) of the image |
| `SHA256SUMS`, `SHA256SUMS.sigstore.json` | Checksums and signature of the binary archives |

A digest (`sha256:…`) is the fingerprint of the image content. Container
Manager / Container Station download exactly the image with this fingerprint;
a different image cannot have the same digest.

## Prerequisites

- [cosign](https://docs.sigstore.dev/cosign/system_config/installation/)
  version 2.4 or newer (`brew install cosign`, or the binary from the cosign
  release page)
- The release version you want to install, e.g. `v0.4.0` (replace it in the
  commands below)

## 1. Download the files

From the release page `https://github.com/ohm-meter/hallido/releases`,
download `compose.yaml`, `compose.yaml.sha256` and
`compose.yaml.sigstore.json` into one folder, then open a terminal there.

## 2. Check the checksum

```bash
shasum -a 256 -c compose.yaml.sha256
```

Expected: `compose.yaml: OK`.

## 3. Check the signature of compose.yaml

```bash
VERSION=v0.4.0
cosign verify-blob compose.yaml \
  --bundle compose.yaml.sigstore.json \
  --certificate-identity "https://github.com/ohm-meter/hallido/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Expected: `Verified OK`. This proves that the file was produced by our
release workflow for exactly this tag.

## 4. Check the signature of the image

Take the image reference from `compose.yaml`:

```bash
IMAGE=$(sed -n 's/^ *image: *//p' compose.yaml)
echo "$IMAGE"
```

It must start with `ghcr.io/ohm-meter/hallido@sha256:`. Then:

```bash
cosign verify "$IMAGE" \
  --certificate-identity "https://github.com/ohm-meter/hallido/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Expected: a JSON block and the lines “The cosign claims were validated”,
“Existence of the claims in the transparency log was verified offline” and
“The code-signing certificate was verified”.

Optional — the signed SBOM (list of everything in the image):

```bash
cosign verify-attestation "$IMAGE" --type spdxjson \
  --certificate-identity "https://github.com/ohm-meter/hallido/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com > /dev/null && echo "SBOM OK"
```

## 5. Compare with the NAS

After the installation, check that the NAS runs the same image:

- **Synology Container Manager:** Container → `hallido` → Details → the
  image name shows the digest.
- **QNAP Container Station:** Applications → `hallido` → the YAML shows
  the image with the digest.

The digest must be the same as in step 4.

## If a check fails

Do **not** install. Download the files again directly from
`github.com/ohm-meter/hallido/releases` (not from a mirror or another
website). If it still fails, report it privately via **Security → Report a
vulnerability** in the repository.

## See also

- [What Hallido does on your network and your NAS](../explanation/what-hallido-does.md)
- [Install on a NAS](../nas/en/README.md)
