#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

chmod +x "$repo_root/.githooks/pre-commit"
chmod +x "$repo_root/scripts/scan-secrets.sh"

git -C "$repo_root" config core.hooksPath .githooks

cat <<'EOF'
Git hooks installed for this repository.
- pre-commit now runs gitleaks against staged changes.
- to bypass once (not recommended): METACLAW_SKIP_SECRET_SCAN=1 git commit ...
EOF
