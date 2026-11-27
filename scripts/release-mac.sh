#!/usr/bin/env bash
# Build and Developer ID sign + Apple notarize StopThat.app & StopThat.dmg
# on Apple Silicon macOS.
#
# Automatically uses the dedicated actnow-signing keychain and actnow-notary
# profile (by SHA-1, zero GUI prompts over SSH) when present; otherwise falls
# back to MACOS_SIGNING_IDENTITY / NOTARY_PROFILE environment variables.
#
# Usage:
#   scripts/release-mac.sh                 # Developer ID signed + notarized
#   scripts/release-mac.sh --skip-notarize # Developer ID signed, skip notarization
#   scripts/release-mac.sh --dry-run       # ad-hoc signed .app + .dmg
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

dry=false
notarize=true
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry=true ;;
    --skip-notarize) notarize=false ;;
    *) echo "unknown option: $arg" >&2; exit 2 ;;
  esac
done

fail() { echo "release-mac: $*" >&2; exit 1; }
[[ "$(uname -s)" == "Darwin" ]] || fail "must run on macOS"
export PATH="/opt/homebrew/bin:$PATH"
command -v jq > /dev/null || fail "needs jq"
command -v go > /dev/null || fail "needs go"

ACTNOW_KEYCHAIN="$HOME/Library/Keychains/actnow-signing.keychain-db"
ACTNOW_KCPASS="$HOME/.config/actnow/signing/keychain-password"

if $dry; then
  unset MACOS_SIGNING_IDENTITY NOTARY_PROFILE NOTARY_KEYCHAIN
  notarize=false
else
  # Prefer dedicated actnow-signing keychain if installed on this Mac.
  if [[ -z "${MACOS_SIGNING_IDENTITY:-}" && -f "$ACTNOW_KEYCHAIN" ]]; then
    if [[ -f "$ACTNOW_KCPASS" ]]; then
      security unlock-keychain -p "$(cat "$ACTNOW_KCPASS")" "$ACTNOW_KEYCHAIN"
      security set-keychain-settings -lut 7200 "$ACTNOW_KEYCHAIN"
    fi
    sha="$(security find-identity -v -p codesigning "$ACTNOW_KEYCHAIN" \
      | awk '/Developer ID Application:/ && length($2) == 40 && $2 ~ /^[0-9A-F]+$/ { print $2; exit }')"
    [[ -n "$sha" ]] || fail "no valid Developer ID Application identity in $ACTNOW_KEYCHAIN"
    MACOS_SIGNING_IDENTITY="$sha"
    if $notarize; then
      NOTARY_PROFILE="${NOTARY_PROFILE:-actnow-notary}"
      NOTARY_KEYCHAIN="${NOTARY_KEYCHAIN:-$ACTNOW_KEYCHAIN}"
    fi
  elif [[ -z "${MACOS_SIGNING_IDENTITY:-}" ]]; then
    ids="$(security find-identity -v -p codesigning | sed -n 's/.*"\(Developer ID Application: .*\)"/\1/p' | sort -u)"
    if [[ -n "${APPLE_TEAM_ID:-}" ]]; then
      ids="$(grep -F "($APPLE_TEAM_ID)" <<<"$ids" || true)"
    fi
    [[ -n "$ids" ]] || fail "no Developer ID Application identity found in keychains"
    [[ "$(wc -l <<<"$ids")" -eq 1 ]] || fail "multiple Developer ID identities; set APPLE_TEAM_ID or MACOS_SIGNING_IDENTITY"
    MACOS_SIGNING_IDENTITY="$ids"
    if $notarize; then
      NOTARY_PROFILE="${NOTARY_PROFILE:-notary}"
    fi
  fi

  export MACOS_SIGNING_IDENTITY
  if $notarize; then
    export NOTARY_PROFILE="${NOTARY_PROFILE:-actnow-notary}"
    export NOTARY_KEYCHAIN="${NOTARY_KEYCHAIN:-}"
  else
    unset NOTARY_PROFILE NOTARY_KEYCHAIN
  fi
fi

backup="$(mktemp)"
cp mygo.json "$backup"
trap 'cp "$backup" mygo.json; rm -f "$backup"' EXIT

if [[ -n "${MACOS_SIGNING_IDENTITY:-}" || -n "${NOTARY_PROFILE:-}" ]]; then
  updated="$(jq \
    --arg identity "${MACOS_SIGNING_IDENTITY:-}" \
    --arg profile "${NOTARY_PROFILE:-}" \
    --arg keychain "${NOTARY_KEYCHAIN:-}" '
    (if $identity != "" then .macos.signingIdentity = $identity else . end)
    | (if $profile != "" then .macos.notarize = ({keychainProfile: $profile}
        + (if $keychain != "" then {keychain: $keychain} else {} end)) else . end)
  ' mygo.json)"
  printf '%s\n' "$updated" > mygo.json
fi

echo "release-mac: signing=${MACOS_SIGNING_IDENTITY:-ad-hoc} notarize=$notarize profile=${NOTARY_PROFILE:-none}"

flags=()
$notarize || flags+=(-skip-notarize)
rm -rf build/darwin-arm64
go tool mygo build -platform darwin/arm64 ${flags[@]+"${flags[@]}"}

app_path="$(find build -maxdepth 3 -name 'StopThat.app' -type d | head -n 1)"
[[ -n "$app_path" ]] || fail "StopThat.app not found in build/"
echo "release-mac: verifying $app_path"
codesign --verify --deep --strict --verbose=2 "$app_path"

if ! $dry && $notarize; then
  spctl --assess --type execute --verbose "$app_path"
fi

ls -lh build/darwin-arm64 2>/dev/null || true
echo "release-mac: DONE"
