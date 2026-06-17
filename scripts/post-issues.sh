#!/usr/bin/env bash
#
# post-issues.sh — Deterministically post Markdown issue files to GitHub with `gh`.
#
# How it works
# ------------
# Each file in the issues directory (default: <repo>/issues/*.md) is a GitHub
# issue described by YAML-ish front-matter plus a Markdown body:
#
#     ---
#     title: "[Task]: Do the thing"
#     labels: task, enhancement
#     uploaded:
#     ---
#     ## What needs to be done
#     ...body...
#
# The script:
#   1. Ensures every label referenced across the files exists (creates missing ones).
#   2. Uploads every file whose `uploaded:` field is empty, then writes the new
#      issue URL back into that field — so re-running NEVER creates duplicates.
#   3. Resolves cross-issue links: a `{{issue:other-file.md}}` token anywhere in a
#      body is replaced with the `#<number>` of that file's issue. Because the
#      referenced issue may be uploaded later in the same run, a second pass edits
#      any issue that still contains unresolved tokens once all numbers are known,
#      then bakes the resolved `#N` back into the source file.
#   4. Deletes each source file once its issue is on GitHub, so posted issues
#      don't linger as markdown in source control (set KEEP_UPLOADED=1 to keep
#      them). Files that fail to upload are kept for retry.
#
# This makes the script reusable for ANY folder of issue files. Note: because
# posted files are deleted by default, `{{issue:...}}` links only resolve to
# issues posted in the SAME run — a token pointing at a file deleted by a prior
# run can't be resolved (use the literal `#N`, or run KEEP_UPLOADED=1).
#
# Usage:
#   ./post-issues.sh                      # post unuploaded issues in <repo>/issues
#   ISSUES_DIR=path/to/issues ./post-issues.sh
#   REPO=owner/name ./post-issues.sh      # target a specific repo (else gh infers)
#   DRY_RUN=1 ./post-issues.sh            # show what would happen, change nothing
#   KEEP_UPLOADED=1 ./post-issues.sh      # keep source files after posting
#
# Requires: gh (authenticated), awk, sed, grep.

set -euo pipefail

DRY_RUN="${DRY_RUN:-0}"

# After an issue is posted, delete its source markdown so posted issues don't
# linger in source control. Files that FAIL to upload (no `uploaded:` URL) are
# always kept so they can be fixed and retried. Set KEEP_UPLOADED=1 to retain
# everything (the old behavior).
KEEP_UPLOADED="${KEEP_UPLOADED:-0}"

# Resolve the issues directory: explicit override, else <repo-root>/issues,
# else <script-dir>/../issues as a fallback when not in a git repo.
if [[ -n "${ISSUES_DIR:-}" ]]; then
    ISSUES_DIR="$ISSUES_DIR"
elif root="$(git rev-parse --show-toplevel 2>/dev/null)"; then
    ISSUES_DIR="$root/issues"
else
    ISSUES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../issues" && pwd)"
fi

# Pass `--repo owner/name` to gh only if REPO is set; otherwise gh infers it.
REPO_ARGS=()
if [[ -n "${REPO:-}" ]]; then
    REPO_ARGS=(--repo "$REPO")
fi

# preflight_auth — make sure gh can actually WRITE to the repo before we start.
#
# gh prioritizes the GITHUB_TOKEN/GH_TOKEN env vars over its stored login. A
# token without the `repo` scope can still READ (so `gh label list` succeeds) but
# every WRITE (label/issue create) comes back as HTTP 404 — GitHub returns 404
# rather than 403 for unauthorized writes. That makes the failure look like a
# missing repo. Detect a scope-less env token and fall back to the stored login.
preflight_auth() {
    if [[ -n "${GH_TOKEN:-}${GITHUB_TOKEN:-}" ]]; then
        local scopes
        scopes="$(gh api -i user 2>/dev/null \
            | awk -F': ' 'tolower($1)=="x-oauth-scopes"{print $2}' | tr -d ' \r')"
        if [[ ",${scopes}," != *",repo,"* ]]; then
            echo "warning: ignoring GH_TOKEN/GITHUB_TOKEN — missing 'repo' scope (scopes: '${scopes:-none}')" >&2
            echo "         falling back to gh's stored login (run 'gh auth login' if this fails)." >&2
            unset GH_TOKEN GITHUB_TOKEN
        fi
    fi

    if ! gh auth status >/dev/null 2>&1; then
        echo "error: gh is not authenticated. Run: gh auth login" >&2
        exit 1
    fi
}

# --------------------------------------------------------------------------
# Front-matter helpers (no external YAML parser; just first `---`...`---` block)
# --------------------------------------------------------------------------

# fm_value <file> <key> — print the trimmed value of a front-matter key (or empty).
fm_value() {
    awk -v key="$2" '
        NR==1 && $0!="---" { exit }          # no front-matter at all
        $0=="---" { c++; next }
        c==1 {
            idx=index($0,":")
            if (idx>0) {
                k=substr($0,1,idx-1); gsub(/^[ \t]+|[ \t]+$/,"",k)
                if (k==key) {
                    v=substr($0,idx+1); gsub(/^[ \t]+|[ \t]+$/,"",v)
                    gsub(/^"|"$/,"",v)        # strip optional surrounding quotes
                    print v; exit
                }
            }
        }
        c>=2 { exit }
    ' "$1"
}

# fm_block <file> — print the front-matter block including both `---` fences.
fm_block() {
    awk '
        NR==1 && $0!="---" { exit }
        { print }
        $0=="---" { c++; if (c==2) exit }
    ' "$1"
}

# fm_body <file> — print everything after the front-matter (or the whole file
# if there is no front-matter).
fm_body() {
    awk '
        NR==1 && $0!="---" { nofm=1 }
        nofm { print; next }
        $0=="---" { c++; next }
        c>=2 { print }
    ' "$1"
}

# issue_number <file> — derive the issue number from the stored `uploaded:` URL.
issue_number() {
    local url; url="$(fm_value "$1" uploaded)"
    [[ -n "$url" ]] && basename "$url" || true
}

# set_uploaded <file> <url> — write/replace the `uploaded:` front-matter field.
set_uploaded() {
    local file="$1" url="$2" tmp; tmp="$(mktemp)"
    awk -v val="$url" '
        NR==1 && $0!="---" { print; nofm=1; next }
        nofm { print; next }
        $0=="---" {
            c++
            if (c==2 && !done) { print "uploaded: " val; done=1 }
            print; next
        }
        c==1 {
            idx=index($0,":"); k=substr($0,1,idx-1); gsub(/^[ \t]+|[ \t]+$/,"",k)
            if (k=="uploaded") { print "uploaded: " val; done=1; next }
        }
        { print }
    ' "$file" >"$tmp"
    mv "$tmp" "$file"
}

# --------------------------------------------------------------------------
# Token resolution: {{issue:FILE}} -> #<number>
# --------------------------------------------------------------------------

# resolve_tokens <infile> <outfile> — copy in->out replacing every resolvable
# {{issue:FILE}} token. Returns 0 if ALL tokens were resolved, 1 if some remain.
resolve_tokens() {
    cp "$1" "$2"
    local unresolved=0 tok file num pat
    while IFS= read -r tok; do
        [[ -z "$tok" ]] && continue
        file="${tok#\{\{issue:}"; file="${file%\}\}}"
        num="$(issue_number "$ISSUES_DIR/$file" 2>/dev/null || true)"
        if [[ -n "$num" ]]; then
            pat="$(printf '%s' "$tok" | sed 's/[.[\*^$/]/\\&/g')"
            sed -i "s|$pat|#$num|g" "$2"
        else
            unresolved=1
        fi
    done < <(grep -oE '\{\{issue:[^}]+\}\}' "$2" | sort -u || true)
    return "$unresolved"
}

# --------------------------------------------------------------------------
# Labels
# --------------------------------------------------------------------------

label_color() {
    case "$1" in
        epic) echo "5319e7" ;;
        roadmap) echo "0e8a16" ;;
        task) echo "fbca04" ;;
        bug) echo "d73a4a" ;;
        enhancement) echo "a2eeef" ;;
        documentation) echo "0075ca" ;;
        *) echo "ededed" ;;
    esac
}

ensure_label() {
    local name="$1"
    if gh label list "${REPO_ARGS[@]}" --limit 500 | grep -qiE "^${name}[[:space:]]"; then
        return 0
    fi
    echo "creating missing label: $name"
    if [[ "$DRY_RUN" == "1" ]]; then
        echo "  DRY_RUN: gh label create $name --color $(label_color "$name")"
    else
        gh label create "$name" "${REPO_ARGS[@]}" --color "$(label_color "$name")"
    fi
}

# split a "labels:" value ("a, b , c") into a clean newline list
split_labels() {
    printf '%s' "$1" | tr ',' '\n' | sed 's/^[ \t]*//;s/[ \t]*$//' | grep -v '^$' || true
}

# --------------------------------------------------------------------------
# Main
# --------------------------------------------------------------------------

shopt -s nullglob
FILES=("$ISSUES_DIR"/*.md)
shopt -u nullglob

if [[ ${#FILES[@]} -eq 0 ]]; then
    echo "No issue files found in: $ISSUES_DIR" >&2
    exit 1
fi

echo "Issues directory: $ISSUES_DIR"
echo "Found ${#FILES[@]} issue file(s)."
echo

# 0. Make sure we're authenticated with write access before touching the API.
preflight_auth

# 1. Ensure every referenced label exists.
ALL_LABELS="$(for f in "${FILES[@]}"; do split_labels "$(fm_value "$f" labels)"; done | sort -u)"
if [[ -n "$ALL_LABELS" ]]; then
    while IFS= read -r lbl; do
        [[ -n "$lbl" ]] && ensure_label "$lbl"
    done <<<"$ALL_LABELS"
fi

# 2. Upload every file not yet marked uploaded.
created=0 skipped=0
for f in "${FILES[@]}"; do
    name="$(basename "$f")"
    if [[ -n "$(fm_value "$f" uploaded)" ]]; then
        echo "skip (already uploaded): $name"
        ((skipped++)) || true
        continue
    fi

    title="$(fm_value "$f" title)"
    if [[ -z "$title" ]]; then
        echo "skip (no title in front-matter): $name" >&2
        continue
    fi

    label_args=()
    while IFS= read -r lbl; do
        [[ -n "$lbl" ]] && label_args+=(--label "$lbl")
    done < <(split_labels "$(fm_value "$f" labels)")

    body_tmp="$(mktemp)"; resolved_tmp="$(mktemp)"
    fm_body "$f" >"$body_tmp"
    resolve_tokens "$body_tmp" "$resolved_tmp" || true   # leave any unresolved tokens for pass 2

    if [[ "$DRY_RUN" == "1" ]]; then
        echo "DRY_RUN: would create '$title' (labels: ${label_args[*]:-none}) <- $name"
        rm -f "$body_tmp" "$resolved_tmp"
        continue
    fi

    url="$(gh issue create "${REPO_ARGS[@]}" \
        --title "$title" \
        --body-file "$resolved_tmp" \
        "${label_args[@]}")"
    echo "created: $url <- $name"
    set_uploaded "$f" "$url"
    ((created++)) || true
    rm -f "$body_tmp" "$resolved_tmp"
done

# 3. Second pass: resolve cross-issue links that pointed at issues created later
#    in this run. Only touches files whose body still contains tokens AND whose
#    own issue exists. Bakes resolved #N back into the source file so subsequent
#    runs have nothing left to do.
if [[ "$DRY_RUN" != "1" ]]; then
    for f in "${FILES[@]}"; do
        num="$(issue_number "$f")"
        [[ -z "$num" ]] && continue
        body_tmp="$(mktemp)"
        fm_body "$f" >"$body_tmp"
        if ! grep -q '{{issue:' "$body_tmp"; then
            rm -f "$body_tmp"; continue
        fi
        resolved_tmp="$(mktemp)"
        if resolve_tokens "$body_tmp" "$resolved_tmp"; then
            echo "linking issue #$num <- $(basename "$f")"
            gh issue edit "$num" "${REPO_ARGS[@]}" --body-file "$resolved_tmp" >/dev/null
            { fm_block "$f"; cat "$resolved_tmp"; } >"$f.tmp" && mv "$f.tmp" "$f"
        else
            echo "warning: $(basename "$f") still has unresolved {{issue:...}} links" >&2
        fi
        rm -f "$body_tmp" "$resolved_tmp"
    done
fi

# 4. Clean up: remove source files whose issue now lives on GitHub, so posted
#    issues don't accumulate as markdown in the repo. Runs last, after the
#    cross-link pass, so {{issue:...}} resolution within this run still works.
#    Files without an `uploaded:` URL (upload failed / no title) are kept.
removed=0
if [[ "$DRY_RUN" != "1" && "$KEEP_UPLOADED" != "1" ]]; then
    for f in "${FILES[@]}"; do
        [[ -e "$f" ]] || continue
        if [[ -n "$(fm_value "$f" uploaded)" ]]; then
            echo "removing posted source file: $(basename "$f")"
            rm -f "$f"
            ((removed++)) || true
        fi
    done
fi

echo
echo "Done. created=$created skipped=$skipped removed=$removed (dry_run=$DRY_RUN, keep_uploaded=$KEEP_UPLOADED)"
