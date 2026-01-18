# APT Repository (Debian/Ubuntu)

This directory documents how to publish Nexus as a deb package. RPMs are generated via GoReleaser.

## 1) Build a .deb

Use `nfpm` or `fpm`. Example with `nfpm`:

```bash
nfpm package -f packaging/apt/nfpm.yaml -p deb
```

## 2) Publish

- Host `.deb` and `Packages.gz` on a static site (GitHub Pages or S3).
- Create a signed repo with `reprepro` or `aptly`.

## 3) Install

Users can install via:

```bash
curl -fsSL https://YOUR_DOMAIN/KEY.gpg | sudo gpg --dearmor -o /etc/apt/keyrings/nexus.gpg
echo "deb [signed-by=/etc/apt/keyrings/nexus.gpg] https://YOUR_DOMAIN/apt stable main" | sudo tee /etc/apt/sources.list.d/nexus.list
sudo apt update
sudo apt install nexus
```
