#!/usr/bin/env sh
set -eu

mode="${1:---staged}"

if ! command -v gitleaks >/dev/null 2>&1; then
  cat >&2 <<'EOF'
secret-scan: gitleaks is not installed.
Install:
  macOS: brew install gitleaks
  Linux: https://github.com/gitleaks/gitleaks
EOF
  exit 1
fi

has_subcommand() {
  subcmd="$1"
  gitleaks --help 2>&1 | grep -q "[[:space:]]$subcmd[[:space:]]"
}

supports_flag() {
  subcmd="$1"
  flag="$2"
  gitleaks "$subcmd" --help 2>&1 | grep -q -- "$flag"
}

run_git_subcommand() {
  if [ "$mode" = "--staged" ]; then
    if supports_flag git --staged; then
      if supports_flag git --redact; then
        exec gitleaks git --staged --redact
      fi
      exec gitleaks git --staged
    fi
    echo "secret-scan: installed gitleaks 'git' subcommand does not support --staged." >&2
    exit 1
  fi

  if supports_flag git --redact; then
    exec gitleaks git --redact
  fi
  exec gitleaks git
}

run_protect_subcommand() {
  if [ "$mode" = "--staged" ]; then
    if supports_flag protect --staged; then
      if supports_flag protect --redact; then
        exec gitleaks protect --staged --redact
      fi
      exec gitleaks protect --staged
    fi
    echo "secret-scan: installed gitleaks 'protect' subcommand does not support --staged." >&2
    exit 1
  fi

  if supports_flag protect --redact; then
    exec gitleaks protect --redact
  fi
  exec gitleaks protect
}

if has_subcommand git; then
  run_git_subcommand
fi

if has_subcommand protect; then
  run_protect_subcommand
fi

echo "secret-scan: unable to find a supported gitleaks subcommand (expected 'git' or 'protect')." >&2
exit 1
