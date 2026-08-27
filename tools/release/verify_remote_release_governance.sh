#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf '%s\n' \
    'Usage: verify_remote_release_governance.sh --repository OWNER/REPO --release-actor-login LOGIN'
}

repository=''
release_actor_login=''
while (($#)); do
  case "$1" in
    --repository)
      repository=${2:-}
      shift 2
      ;;
    --release-actor-login)
      release_actor_login=${2:-}
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
if [[ ! "${release_actor_login}" =~ ^[A-Za-z0-9-]+$ ]]; then
  echo '--release-actor-login must be one GitHub login' >&2
  exit 2
fi
command -v gh >/dev/null
command -v jq >/dev/null

inspector_json="$(gh api user)"
inspector_login="$(jq -er '.login' <<< "${inspector_json}")"
release_actor_json="$(gh api "/users/${release_actor_login}")"
release_actor_id="$(jq -er '.id' <<< "${release_actor_json}")"
repository_json="$(gh api "/repos/${repository}")"
jq -e '.permissions.admin == true' <<< "${repository_json}" >/dev/null || {
  echo "${inspector_login} must have repository admin access to inspect release governance" >&2
  exit 1
}
repository_id="$(jq -er '.id' <<< "${repository_json}")"

repository_secrets="$(gh api --paginate "/repos/${repository}/actions/secrets?per_page=100")"
if jq -s -e '
  any(.[]; any(.secrets[]; (.name | ascii_upcase) == "CF_API_TOKEN"))
' <<< "${repository_secrets}" >/dev/null; then
  echo 'repository-level CF_API_TOKEN would override the organization secret' >&2
  exit 1
fi

organization_secrets="$(gh api --paginate "/repos/${repository}/actions/organization-secrets?per_page=100")"
jq -s -e '
  ([.[] | .secrets[] | select((.name | ascii_upcase) == "CF_API_TOKEN")] | length) == 1
' <<< "${organization_secrets}" >/dev/null || {
  echo 'CF_API_TOKEN must be available to this repository from organization Actions secrets' >&2
  exit 1
}

deploy_keys="$(gh api "/repos/${repository}/keys?per_page=100")"
if jq -e 'any(.[]; .title == "mss-root-tag-promotion")' \
  <<< "${deploy_keys}" >/dev/null; then
  echo 'the retired Root promotion DeployKey is still present' >&2
  exit 1
fi

environments="$(gh api "/repos/${repository}/environments?per_page=100")"
if jq -e 'any(.environments[]; .name == "root-promotion")' \
  <<< "${environments}" >/dev/null; then
  echo 'the retired root-promotion environment is still present' >&2
  exit 1
fi

actions_variables="$(gh api "/repos/${repository}/actions/variables?per_page=100")"
if jq -e 'any(.variables[]; .name == "RELEASE_READINESS_RUN_ID")' \
  <<< "${actions_variables}" >/dev/null; then
  echo 'the retired RELEASE_READINESS_RUN_ID repository variable is still present' >&2
  exit 1
fi

reject_environment_variable() {
  local environment_name=$1
  local variable_name=$2
  local variables

  variables="$(gh api "/repositories/${repository_id}/environments/${environment_name}/variables?per_page=100")"
  if jq -e --arg name "${variable_name}" \
    'any(.variables[]; .name == $name)' <<< "${variables}" >/dev/null; then
    echo "the retired ${variable_name} variable is still present in ${environment_name}" >&2
    exit 1
  fi
}

for environment_name in release release-v6 release-auto release-v6-auto npm-auto prod; do
  reject_environment_variable "${environment_name}" RELEASE_READINESS_RUN_ID
done

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
  || "${creation_names[1]}" != "v1.3.5-stopped-tags-never-create" ]]; then
  echo 'exactly the consolidated controlled-creation and v1.3.5 stop rulesets may govern release-tag creation' >&2
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

controlled_id="$(unique_ruleset_id release-tags-controlled-creation)"
controlled_ruleset="$(gh api "/repos/${repository}/rulesets/${controlled_id}?includes_parents=true")"
jq -e \
  --arg repository "${repository}" \
  --argjson release_actor_id "${release_actor_id}" '
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
  ([.rules[].type] == ["creation"]) and
  .bypass_actors == [{
    actor_id: $release_actor_id,
    actor_type: "User",
    bypass_mode: "always"
  }]
' <<< "${controlled_ruleset}" >/dev/null || {
  echo 'Root, component, and Docs creation authority must belong only to the explicit release actor' >&2
  exit 1
}

stopped_id="$(unique_ruleset_id v1.3.5-stopped-tags-never-create)"
stopped_ruleset="$(gh api "/repos/${repository}/rulesets/${stopped_id}?includes_parents=true")"
jq -e \
  --arg repository "${repository}" '
  ([
    "refs/tags/admin/v1.3.5",
    "refs/tags/docs/v1.3.5",
    "refs/tags/mss-boot/v1.3.5",
    "refs/tags/v1.3.5",
    "refs/tags/web/antd/v1.3.5",
    "refs/tags/web/antd-v6/v1.3.5"
  ] | sort) as $expected_refs |
  .source_type == "Repository" and
  .source == $repository and
  .target == "tag" and
  .enforcement == "active" and
  (.conditions.ref_name.include | sort) == $expected_refs and
  .conditions.ref_name.exclude == [] and
  .bypass_actors == [] and
  ([.rules[].type] == ["creation"])
' <<< "${stopped_ruleset}" >/dev/null || {
  echo 'v1.3.5 stopped-tag creation must be blocked by the exact no-bypass ruleset' >&2
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

verify_active_environment() {
  local environment_name=$1
  shift
  local environment
  local branch_policies
  local expected_policies

  expected_policies="$({
    printf '%s\n' "$@"
  } | jq -Rsc '
    split("\n") |
    map(select(length > 0) | split("|") | {name: .[0], type: .[1]}) |
    sort_by(.name, .type)
  ')"

  environment="$(gh api "/repos/${repository}/environments/${environment_name}")"
  jq -e \
    --arg environment_name "${environment_name}" '
    .name == $environment_name and
    .can_admins_bypass == false and
    .deployment_branch_policy.protected_branches == false and
    .deployment_branch_policy.custom_branch_policies == true and
    ([.protection_rules[].type] | sort) == ["branch_policy"] and
    ([.protection_rules[] | select(.type == "required_reviewers")] | length) == 0
  ' <<< "${environment}" >/dev/null || {
    echo "${environment_name} environment must have no required reviewers and no administrator bypass" >&2
    exit 1
  }

  branch_policies="$(gh api "/repos/${repository}/environments/${environment_name}/deployment-branch-policies?per_page=100")"
  jq -e \
    --argjson expected_policies "${expected_policies}" '
    .total_count == ($expected_policies | length) and
    (.branch_policies | length) == ($expected_policies | length) and
    ([.branch_policies[] | {name, type}] | sort_by(.name, .type)) == $expected_policies
  ' <<< "${branch_policies}" >/dev/null || {
    echo "${environment_name} environment deployment branch or tag policies are not exact" >&2
    exit 1
  }
}

verify_retired_environment() {
  local environment_name=$1
  local environment
  local branch_policies

  environment="$(gh api "/repos/${repository}/environments/${environment_name}")"
  jq -e \
    --arg environment_name "${environment_name}" '
    .name == $environment_name and
    .can_admins_bypass == false and
    .deployment_branch_policy.protected_branches == false and
    .deployment_branch_policy.custom_branch_policies == true and
    ([.protection_rules[] | select(.type == "branch_policy")] | length) == 1
  ' <<< "${environment}" >/dev/null || {
    echo "${environment_name} must remain a non-bypassable retired environment" >&2
    exit 1
  }

  branch_policies="$(gh api "/repos/${repository}/environments/${environment_name}/deployment-branch-policies?per_page=100")"
  jq -e '
    .total_count == 0 and (.branch_policies | length) == 0
  ' <<< "${branch_policies}" >/dev/null || {
    echo "${environment_name} must allow no branch or tag deployments" >&2
    exit 1
  }
}

verify_retired_environment release
verify_retired_environment release-v6
verify_active_environment release-auto \
  'admin/v*|tag' \
  'mss-boot/v*|tag' \
  'v*|tag'
verify_active_environment release-v6-auto \
  'web/antd-v6/v*|tag'
verify_active_environment npm-auto \
  'v*|tag'
verify_active_environment prod \
  'docs/v*|tag'

verify_environment_secrets() {
  local environment_name=$1
  shift
  local environment_secrets
  local expected_names

  environment_secrets="$(gh api "/repositories/${repository_id}/environments/${environment_name}/secrets?per_page=100")"
  expected_names="$({ printf '%s\n' "$@"; } | jq -Rsc 'split("\n") | map(select(length > 0)) | sort')"
  jq -e \
    --argjson expected_names "${expected_names}" '
    .total_count == ($expected_names | length) and
    ([.secrets[].name] | sort) == $expected_names
  ' <<< "${environment_secrets}" >/dev/null || {
    echo "${environment_name} environment secret names are not exact" >&2
    exit 1
  }
}

verify_environment_secrets release
verify_environment_secrets release-v6
verify_environment_secrets release-auto
verify_environment_secrets release-v6-auto
verify_environment_secrets npm-auto
verify_environment_secrets prod

jq -n \
  --arg repository "${repository}" \
  --arg inspector "${inspector_login}" \
  --arg actor "${release_actor_login}" \
  --argjson controlled_ruleset_id "${controlled_id}" \
  --argjson stopped_ruleset_id "${stopped_id}" \
  --argjson immutable_ruleset_id "${immutable_id}" \
  '{
    success: true,
    repository: $repository,
    inspector: $inspector,
    releaseActor: $actor,
    controlledCreationRuleset: $controlled_ruleset_id,
    stoppedV135CreationRuleset: $stopped_ruleset_id,
    immutableRuleset: $immutable_ruleset_id,
    retiredResources: {
      rootPromotionDeployKey: false,
      rootPromotionEnvironment: false,
      readinessRunVariables: [],
      npmToken: false
    },
    environments: {
      release: [],
      releaseV6: [],
      releaseAuto: ["refs/tags/admin/v*", "refs/tags/mss-boot/v*", "refs/tags/v*"],
      releaseV6Auto: ["refs/tags/web/antd-v6/v*"],
      npmAuto: ["refs/tags/v*"],
      prod: ["refs/tags/docs/v*"]
    },
    environmentSecrets: {
      release: [],
      releaseV6: [],
      releaseAuto: [],
      releaseV6Auto: [],
      npmAuto: [],
      prod: []
    },
    docsCredential: {
      name: "CF_API_TOKEN",
      source: "organization",
      repositoryOverride: false,
      environmentOverride: false
    }
  }'
