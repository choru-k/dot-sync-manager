#!/usr/bin/env bash
# Generate a concise report for GitHub PR comments and reviews using gh + jq.
set -euo pipefail

sanitize_text() {
  local text="$1"
  text="$(printf '%s' "$text" | sed -E 's/!\[([^]]*)\]\([^)]*\)/\1/g' | LC_ALL=C tr -cd '\11\12\40-\176')"
  printf '%s' "$text"
}

print_block() {
  local body
  body="$(sanitize_text "$1")"
  if [[ -z "${body}" ]]; then
    printf 'Description: (no description)\n'
  elif [[ "${body}" != *$'\n'* ]]; then
    printf 'Description: %s\n' "${body}"
  else
    printf 'Description:\n'
    while IFS= read -r line || [[ -n "${line}" ]]; do
      printf '  %s\n' "${line}"
    done <<< "${body}"
  fi
}

print_entry() {
  local source="$1" reviewer="$2" description="$3"
  printf 'Source: %s\n' "${source}"
  printf 'Reviewer: %s\n' "${reviewer}"
  print_block "${description}"
}

process_issue_comment() {
  local owner="$1" repo="$2" identifier="$3" url="$4"
  local json
  json="$(gh api "repos/${owner}/${repo}/issues/comments/${identifier}")"
  local reviewer description
  reviewer="$(jq -r '.user.login' <<< "${json}")"
  description="$(jq -r '.body // ""' <<< "${json}")"

  print_entry "${url}" "${reviewer}" "${description}"
}

process_review() {
  local owner="$1" repo="$2" pr_number="$3" identifier="$4" url="$5"
  local review_json comments_json
  review_json="$(gh api "repos/${owner}/${repo}/pulls/${pr_number}/reviews/${identifier}")"
  comments_json="$(gh api "repos/${owner}/${repo}/pulls/${pr_number}/reviews/${identifier}/comments")"

  local reviewer description
  reviewer="$(jq -r '.user.login' <<< "${review_json}")"
  description="$(jq -r '.body // ""' <<< "${review_json}")"

  print_entry "${url}" "${reviewer}" "${description}"

  if [[ "$(jq 'length' <<< "${comments_json}")" == "0" ]]; then
    printf 'Review Comments: (none)\n'
    return
  fi

  printf 'Review Comments:\n'
  while IFS= read -r comment_json; do
    local comment_id path body_raw body location comment_detail start_line end_line position_value
    comment_id="$(jq -r '.id' <<< "${comment_json}")"
    path="$(jq -r '.path // "(no file)"' <<< "${comment_json}")"
    body_raw="$(jq -r '.body // ""' <<< "${comment_json}")"
    body="$(sanitize_text "${body_raw}")"
    comment_detail="$(gh api "repos/${owner}/${repo}/pulls/comments/${comment_id}" 2>/dev/null || echo '{}')"
    start_line="$(jq -r '.start_line // .original_start_line // empty' <<< "${comment_detail}")"
    end_line="$(jq -r '.line // .original_line // empty' <<< "${comment_detail}")"
    position_value="$(jq -r '.position // .original_position // empty' <<< "${comment_detail}")"
    if [[ "${start_line}" == "null" ]]; then start_line=""; fi
    if [[ "${end_line}" == "null" ]]; then end_line=""; fi
    if [[ "${position_value}" == "null" ]]; then position_value=""; fi

    if [[ -n "${start_line}" && -n "${end_line}" ]]; then
      if [[ "${start_line}" != "${end_line}" ]]; then
        location="${path}:${start_line}-${end_line}"
      else
        location="${path}:${end_line}"
      fi
    elif [[ -n "${end_line}" ]]; then
      location="${path}:${end_line}"
    elif [[ -n "${start_line}" ]]; then
      location="${path}:${start_line}"
    elif [[ -n "${position_value}" ]]; then
      location="${path}@pos${position_value}"
    else
      location="${path}"
    fi

    if [[ -z "${body}" ]]; then
      printf -- '- %s - (no comment body)\n' "${location}"
    elif [[ "${body}" == *$'\n'* ]]; then
      printf -- '- %s\n' "${location}"
      while IFS= read -r line || [[ -n "${line}" ]]; do
        printf '  %s\n' "${line}"
      done <<< "${body}"
    else
      printf -- '- %s - %s\n' "${location}" "${body}"
    fi
  done < <(jq -c '.[]' <<< "${comments_json}")
}

process_pull_request() {
  local owner="$1" repo="$2" pr_number="$3" url="$4"

  printf '== Issue Comments ==\n'
  local issue_comments_json
  issue_comments_json="$(gh api "repos/${owner}/${repo}/issues/${pr_number}/comments")"
  local issue_count
  issue_count="$(jq 'length' <<< "${issue_comments_json}")"
  if [[ "${issue_count}" == "0" ]]; then
    printf '(none)\n'
  else
    local idx=0
    while IFS= read -r comment_json; do
      idx=$(( idx + 1 ))
      local source reviewer description
      source="$(jq -r '.html_url' <<< "${comment_json}")"
      reviewer="$(jq -r '.user.login' <<< "${comment_json}")"
      description="$(jq -r '.body // ""' <<< "${comment_json}")"
      print_entry "${source}" "${reviewer}" "${description}"
      if (( idx != issue_count )); then
        printf '\n'
      fi
    done < <(jq -c '.[]' <<< "${issue_comments_json}")
  fi

  printf '\n== Reviews ==\n'
  local reviews_json
  reviews_json="$(gh api "repos/${owner}/${repo}/pulls/${pr_number}/reviews")"
  local review_count
  review_count="$(jq 'length' <<< "${reviews_json}")"
  if [[ "${review_count}" == "0" ]]; then
    printf '(none)\n'
  else
    local r_idx=0
    while IFS= read -r review_line; do
      r_idx=$(( r_idx + 1 ))
      local review_id review_url
      review_id="$(jq -r '.id' <<< "${review_line}")"
      review_url="$(jq -r '.html_url' <<< "${review_line}")"
      process_review "${owner}" "${repo}" "${pr_number}" "${review_id}" "${review_url}"
      if (( r_idx != review_count )); then
        printf '\n'
      fi
    done < <(jq -c '.[]' <<< "${reviews_json}")
  fi
}

process_url() {
  local url="$1"
  local fragment_regex='^https://github\.com/([^/]+)/([^/]+)/pull/([0-9]+)#(issuecomment|pullrequestreview)-([0-9]+)$'
  local pr_regex='^https://github\.com/([^/]+)/([^/]+)/pull/([0-9]+)/*$'

  if [[ "${url}" =~ ${fragment_regex} ]]; then
    local owner="${BASH_REMATCH[1]}"
    local repo="${BASH_REMATCH[2]}"
    local pr_number="${BASH_REMATCH[3]}"
    local kind="${BASH_REMATCH[4]}"
    local identifier="${BASH_REMATCH[5]}"
    if [[ "${kind}" == "issuecomment" ]]; then
      process_issue_comment "${owner}" "${repo}" "${identifier}" "${url}"
    else
      process_review "${owner}" "${repo}" "${pr_number}" "${identifier}" "${url}"
    fi
  elif [[ "${url}" =~ ${pr_regex} ]]; then
    local owner="${BASH_REMATCH[1]}"
    local repo="${BASH_REMATCH[2]}"
    local pr_number="${BASH_REMATCH[3]}"
    process_pull_request "${owner}" "${repo}" "${pr_number}" "${url}"
  else
    printf 'Error: unsupported URL format: %s\n' "${url}" >&2
    return 1
  fi
}

main() {
  if ! command -v gh >/dev/null 2>&1; then
    printf 'Error: gh CLI is required.\n' >&2
    return 1
  fi
  if ! command -v jq >/dev/null 2>&1; then
    printf 'Error: jq is required.\n' >&2
    return 1
  fi

  local urls=()
  if (( $# > 0 )); then
    local arg
    for arg in "$@"; do
      [[ -n "${arg}" ]] && urls+=("${arg}")
    done
  else
    local line
    while IFS= read -r line; do
      [[ -n "${line}" ]] && urls+=("${line}")
    done
  fi

  if (( ${#urls[@]} == 0 )); then
    printf 'Usage: %s <url> [<url> ...]\n' "${0##*/}" >&2
    printf '       cat urls.txt | %s\n' "${0##*/}" >&2
    return 1
  fi

  local idx last_index status=0
  last_index=$(( ${#urls[@]} - 1 ))
  for idx in "${!urls[@]}"; do
    if ! process_url "${urls[idx]}"; then
      status=1
      break
    fi
    if (( idx != last_index )); then
      printf '\n'
    fi
  done
  return "${status}"
}

main "$@"
