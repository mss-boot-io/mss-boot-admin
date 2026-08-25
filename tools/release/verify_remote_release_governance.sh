#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf '%s\n' \
    'Usage: verify_remote_release_governance.sh --repository OWNER/REPO --reviewer-login LOGIN'
}

repository=''
reviewer_login=''
while (($#)); do
  case "$1" in
    --repository)
      repository=${2:-}
      shift 2
      ;;
    --reviewer-login)
      reviewer_login=${2:-}
      shift 2
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! "${repository}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo '--repository must be OWNER/REPO' >&2
  exit 2
fi
if [[ ! "${reviewer_login}" =~ ^[A-Za-z0-9-]+$ ]]; then
  echo '--reviewer-login must be one GitHub login' >&2
  exit 2
fi
command -v gh >/dev/null
command -v jq >/dev/null

actor_json="$(gh api user)"
actor_login="$(jq -er '.login' <<< "${actor_json}")"
actor_id="$(jq -er '.id' <<< "${actor_json}")"
reviewer_json="$(gh api "/users/${reviewer_login}")"
reviewer_id="$(jq -er '.id' <<< "${reviewer_json}")"
if [[ "${actor_id}" == "${reviewer_id}" ]]; then
  echo 'release actor and protected-environment reviewer must be different accounts' >&2
  exit 1
fi
repository_json="$(gh api "/repos/${repository}")"
jq -e '.permissions.admin == true' <<< "${repository_json}" >/dev/null || {
  echo "${actor_login} must have repository admin access to inspect bypass actors" >&2
  exit 1
}
repository_id="$(jq -er '.id' <<< "${repository_json}")"

rulesets="$(gh api "/repos/${repository}/rulesets?includes_parents=true&per_page=100")"
mapfile -t creation_names < <(
  while IFS= read -r ruleset_id; do
    ruleset="$(gh api "/repos/${repository}/rulesets/${ruleset_id}?includes_parents=true")"
    if jq -e 'any(.rules[]; .type == "creation")' <<< "${ruleset}" >/dev/null; then
      jq -r '.name' <<< "${ruleset}"
    fi
  done < <(jq -r '.[] | select(
    .target == "tag" and .enforcement == "active"
  ) | .id' <<< "${rulesets}")
)
mapfile -t creation_names < <(printf '%s\n' "${creation_names[@]}" | sort)
if [[ "${#creation_names[@]}" -ne 2 \
  || "${creation_names[0]}" != "release-tags-controlled-creation" \
  || "${creation_names[1]}" != "root-release-tag-controlled-creation" ]]; then
  echo 'exactly the component and root creation rulesets may create release tags' >&2
  exit 1
fi

unique_ruleset_id() {
  local name=$1
  local ids
  ids="$(jq -c --arg name "${name}" '[.[] | select(
    .name == $name and
    .target == "tag" and
    .enforcement == "active"
  ) | .id]' <<< "${rulesets}")"
  if [[ "$(jq -r 'length' <<< "${ids}")" -ne 1 ]]; then
    echo "exactly one active repository tag ruleset named ${name} is required" >&2
    exit 1
  fi
  jq -r '.[0]' <<< "${ids}"
}

root_id="$(unique_ruleset_id root-release-tag-controlled-creation)"
root_ruleset="$(gh api "/repos/${repository}/rulesets/${root_id}?includes_parents=true")"
jq -e \
  --arg repository "${repository}" '
  .source_type == "Repository" and
  .source == $repository and
  .target == "tag" and
  .enforcement == "active" and
  .conditions.ref_name.include == ["refs/tags/v*"] and
  .conditions.ref_name.exclude == [] and
  ([.rules[].type] == ["creation"]) and
  (.bypass_actors == [{
    actor_id: null,
    actor_type: "DeployKey",
    bypass_mode: "always"
  }])
' <<< "${root_ruleset}" >/dev/null || {
  echo 'root creation authority must be root-only and deploy-key-only' >&2
  exit 1
}

deploy_keys="$(gh api "/repos/${repository}/keys?per_page=100")"
jq -e '
  length == 1 and
  .[0].title == "mss-root-tag-promotion" and
  .[0].read_only == false and
  .[0].verified == true and
  .[0].enabled == true and
  (.[0].key | startswith("ssh-ed25519 "))
' <<< "${deploy_keys}" >/dev/null || {
  echo 'repository must have exactly one verified, enabled, write-enabled Ed25519 deploy key titled mss-root-tag-promotion' >&2
  exit 1
}
deploy_key_id="$(jq -er '.[0].id' <<< "${deploy_keys}")"

controlled_id="$(unique_ruleset_id release-tags-controlled-creation)"
controlled_ruleset="$(gh api "/repos/${repository}/rulesets/${controlled_id}?includes_parents=true")"
jq -e \
  --arg repository "${repository}" \
  --argjson actor_id "${actor_id}" \
  --argjson reviewer_id "${reviewer_id}" '
  ([
    "refs/tags/admin/v*",
    "refs/tags/docs/v*",
    "refs/tags/mss-boot/v*",
    "refs/tags/web/antd-v6/v*",
    "refs/tags/web/antd/v*"
  ] | sort) as $expected_refs |
  ([$actor_id, $reviewer_id] | sort) as $expected_actors |
  .source_type == "Repository" and
  .source == $repository and
  .target == "tag" and
  .enforcement == "active" and
  (.conditions.ref_name.include | sort) == $expected_refs and
  .conditions.ref_name.exclude == [] and
  ([.rules[].type] == ["creation"]) and
  ([.bypass_actors[] | select(
    .actor_type == "User" and .bypass_mode == "always"
  ) | .actor_id] | sort) == $expected_actors and
  ([.bypass_actors[] | select(.actor_type != "User")] | length) == 0
' <<< "${controlled_ruleset}" >/dev/null || {
  echo 'component and Docs creation authority is not confined to the two release accounts' >&2
  exit 1
}

immutable_id="$(unique_ruleset_id release-tags-immutable)"
immutable_ruleset="$(gh api "/repos/${repository}/rulesets/${immutable_id}?includes_parents=true")"
jq -e \
  --arg repository "${repository}" '
  ([
    "refs/tags/admin/v*",
    "refs/tags/docs/v*",
    "refs/tags/mss-boot/v*",
    "refs/tags/v*",
    "refs/tags/web/antd-v6/v*",
    "refs/tags/web/antd/v*"
  ] | sort) as $expected_refs |
  .source_type == "Repository" and
  .source == $repository and
  .target == "tag" and
  .enforcement == "active" and
  (.conditions.ref_name.include | sort) == $expected_refs and
  .conditions.ref_name.exclude == [] and
  .bypass_actors == [] and
  ([.rules[].type] | sort) == ["deletion", "non_fast_forward", "update"]
' <<< "${immutable_ruleset}" >/dev/null || {
  echo 'release tag immutability must cover every release tag with no bypass' >&2
  exit 1
}

environment="$(gh api "/repos/${repository}/environments/root-promotion")"
jq -e \
  --argjson reviewer_id "${reviewer_id}" '
  .name == "root-promotion" and
  .can_admins_bypass == false and
  .deployment_branch_policy.protected_branches == false and
  .deployment_branch_policy.custom_branch_policies == true and
  ([.protection_rules[] | select(.type == "required_reviewers")] | length) == 1 and
  ([.protection_rules[] | select(.type == "required_reviewers")][0] |
    .prevent_self_review == true and
    ([.reviewers[] | select(.type == "User") | .reviewer.id] == [$reviewer_id])
  )
' <<< "${environment}" >/dev/null || {
  echo 'root-promotion environment is not protected by the exact second account' >&2
  exit 1
}

branch_policies="$(gh api "/repos/${repository}/environments/root-promotion/deployment-branch-policies?per_page=100")"
jq -e '
  .total_count == 1 and
  (.branch_policies | length) == 1 and
  .branch_policies[0].name == "main" and
  .branch_policies[0].type == "branch"
' <<< "${branch_policies}" >/dev/null || {
  echo 'root-promotion environment must allow only the main branch' >&2
  exit 1
}

environment_secrets="$(gh api "/repositories/${repository_id}/environments/root-promotion/secrets?per_page=100")"
jq -e '
  .total_count == 1 and
  (.secrets | length) == 1 and
  .secrets[0].name == "ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY"
' <<< "${environment_secrets}" >/dev/null || {
  echo 'root-promotion environment must contain only ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY' >&2
  exit 1
}

jq -n \
  --arg repository "${repository}" \
  --arg actor "${actor_login}" \
  --arg reviewer "${reviewer_login}" \
  --argjson root_ruleset_id "${root_id}" \
  --argjson controlled_ruleset_id "${controlled_id}" \
  --argjson immutable_ruleset_id "${immutable_id}" \
  --argjson deploy_key_id "${deploy_key_id}" \
  '{
    success: true,
    repository: $repository,
    releaseActor: $actor,
    protectedReviewer: $reviewer,
    rootCreationRuleset: $root_ruleset_id,
    rootPromotionDeployKey: {
      id: $deploy_key_id,
      title: "mss-root-tag-promotion",
      writeEnabled: true,
      verified: true,
      enabled: true
    },
    rootPromotionEnvironmentSecret: {
      name: "ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY",
      configured: true
    },
    componentCreationRuleset: $controlled_ruleset_id,
    immutableRuleset: $immutable_ruleset_id,
    environment: "root-promotion",
    allowedRef: "refs/heads/main"
  }'
