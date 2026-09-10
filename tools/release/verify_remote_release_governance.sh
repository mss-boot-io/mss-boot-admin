#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf '%s\n' \
    'Usage: verify_remote_release_governance.sh --repository OWNER/REPO --release-actor-login LOGIN [--scope core|docs]'
}

repository=''
release_actor_login=''
scope='core'
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
    --scope)
      scope=${2:-}
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
if [[ "${scope}" != 'core' && "${scope}" != 'docs' ]]; then
  echo '--scope must be core or docs' >&2
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

if [[ "${scope}" == 'docs' ]]; then
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
fi

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

if [[ "${scope}" == 'core' ]]; then
  for environment_name in release release-v6 release-auto release-v6-auto npm-auto; do
    reject_environment_variable "${environment_name}" RELEASE_READINESS_RUN_ID
  done
else
  reject_environment_variable prod RELEASE_READINESS_RUN_ID
fi

rulesets="$(gh api "/repos/${repository}/rulesets?includes_parents=true&per_page=100")"
mapfile -t creation_names < <(
  while IFS= read -r ruleset_id; do
    ruleset="$(gh api "/repos/${repository}/rulesets/${ruleset_id}?includes_parents=true")"
    if jq -e \
      --arg scope "${scope}" '
      def ref_scope($ref):
        ([
          "refs/tags/docs/v*",
          "refs/tags/docs/v1.3.5",
          "refs/tags/docs/v1.3.6"
        ] | index($ref)) as $docs_index |
        ([
          "refs/tags/admin/v*",
          "refs/tags/admin/v1.3.5",
          "refs/tags/admin/v1.3.6",
          "refs/tags/mss-boot/v*",
          "refs/tags/mss-boot/v1.3.5",
          "refs/tags/mss-boot/v1.3.6",
          "refs/tags/v*",
          "refs/tags/v1.3.5",
          "refs/tags/v1.3.6",
          "refs/tags/web/antd-v6/v*",
          "refs/tags/web/antd-v6/v1.3.5",
          "refs/tags/web/antd-v6/v1.3.6",
          "refs/tags/web/antd/v*",
          "refs/tags/web/antd/v1.3.5",
          "refs/tags/web/antd/v1.3.6"
        ] | index($ref)) as $core_index |
        if $docs_index != null then
          "docs"
        elif $core_index != null then
          "core"
        else
          "ambiguous"
        end;
      any(.rules[]; .type == "creation") and
      any(.conditions.ref_name.include[]?;
        ref_scope(.) as $ref_scope |
        $ref_scope == $scope or $ref_scope == "ambiguous"
      )
    ' <<< "${ruleset}" >/dev/null; then
      jq -r '.name' <<< "${ruleset}"
    fi
  done < <(jq -r '.[] | select(
    .target == "tag" and .enforcement == "active"
  ) | .id' <<< "${rulesets}")
)
mapfile -t creation_names < <(printf '%s\n' "${creation_names[@]}" | sort)
if [[ "${#creation_names[@]}" -ne 3 \
  || "${creation_names[0]}" != "release-tags-controlled-creation" \
  || "${creation_names[1]}" != "v1.3.5-stopped-tags-never-create" \
  || "${creation_names[2]}" != "v1.3.6-stopped-tags-never-create" ]]; then
  echo "exactly the consolidated controlled-creation plus v1.3.5 and v1.3.6 stop rulesets may govern ${scope} release-tag creation" >&2
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
  --arg scope "${scope}" \
  --argjson release_actor_id "${release_actor_id}" '
  [
    "refs/tags/admin/v*",
    "refs/tags/mss-boot/v*",
    "refs/tags/v*",
    "refs/tags/web/antd-v6/v*",
    "refs/tags/web/antd/v*"
  ] as $core_refs |
  ["refs/tags/docs/v*"] as $docs_refs |
  def ref_scope($ref):
    if ($docs_refs | index($ref)) != null then
      "docs"
    elif ($core_refs | index($ref)) != null then
      "core"
    else
      "ambiguous"
    end;
  (if $scope == "docs" then $docs_refs else $core_refs end | sort) as $expected_refs |
  .source_type == "Repository" and
  .source == $repository and
  .target == "tag" and
  .enforcement == "active" and
  all(.conditions.ref_name.include[]?; ref_scope(.) != "ambiguous") and
  all(.conditions.ref_name.exclude[]?; ref_scope(.) != "ambiguous") and
  ([.conditions.ref_name.include[] | select(ref_scope(.) == $scope)] | sort) == $expected_refs and
  ([.conditions.ref_name.exclude[] | select(ref_scope(.) == $scope)] | sort) == [] and
  ([.rules[].type] == ["creation"]) and
  .bypass_actors == [{
    actor_id: $release_actor_id,
    actor_type: "User",
    bypass_mode: "always"
  }]
' <<< "${controlled_ruleset}" >/dev/null || {
  echo "${scope} release-tag creation authority must belong only to the explicit release actor" >&2
  exit 1
}

verify_stopped_ruleset() {
  local version=$1
  local ruleset_id
  local ruleset

  ruleset_id="$(unique_ruleset_id "${version}-stopped-tags-never-create")"
  ruleset="$(gh api "/repos/${repository}/rulesets/${ruleset_id}?includes_parents=true")"
  jq -e \
    --arg repository "${repository}" \
    --arg scope "${scope}" \
    --arg version "${version}" '
    [
      "refs/tags/admin/\($version)",
      "refs/tags/mss-boot/\($version)",
      "refs/tags/\($version)",
      "refs/tags/web/antd/\($version)",
      "refs/tags/web/antd-v6/\($version)"
    ] as $core_refs |
    ["refs/tags/docs/\($version)"] as $docs_refs |
    def ref_scope($ref):
      if ($docs_refs | index($ref)) != null then
        "docs"
      elif ($core_refs | index($ref)) != null then
        "core"
      else
        "ambiguous"
      end;
    (if $scope == "docs" then $docs_refs else $core_refs end | sort) as $expected_refs |
    .source_type == "Repository" and
    .source == $repository and
    .target == "tag" and
    .enforcement == "active" and
    all(.conditions.ref_name.include[]?; ref_scope(.) != "ambiguous") and
    all(.conditions.ref_name.exclude[]?; ref_scope(.) != "ambiguous") and
    ([.conditions.ref_name.include[] | select(ref_scope(.) == $scope)] | sort) == $expected_refs and
    ([.conditions.ref_name.exclude[] | select(ref_scope(.) == $scope)] | sort) == [] and
    .bypass_actors == [] and
    ([.rules[].type] | sort) == ["creation", "deletion", "non_fast_forward", "update"]
  ' <<< "${ruleset}" >/dev/null || {
    echo "${version} stopped tags must be creation- and mutation-frozen by the exact no-bypass ruleset" >&2
    exit 1
  }
  printf '%s\n' "${ruleset_id}"
}

stopped_v135_id="$(verify_stopped_ruleset v1.3.5)"
stopped_v136_id="$(verify_stopped_ruleset v1.3.6)"

immutable_id="$(unique_ruleset_id release-tags-immutable)"
immutable_ruleset="$(gh api "/repos/${repository}/rulesets/${immutable_id}?includes_parents=true")"
docs_deletion_id=''
docs_no_update_id=''
jq -e \
  --arg repository "${repository}" '
  [
    "refs/tags/admin/v*",
    "refs/tags/mss-boot/v*",
    "refs/tags/v*",
    "refs/tags/web/antd-v6/v*",
    "refs/tags/web/antd/v*"
  ] as $expected_refs |
  .source_type == "Repository" and
  .source == $repository and
  .target == "tag" and
  .enforcement == "active" and
  ([.conditions.ref_name.include[]] | sort) == ($expected_refs | sort) and
  ([.conditions.ref_name.exclude[]] | sort) == [] and
  .bypass_actors == [] and
  ([.rules[].type] | sort) == ["deletion", "non_fast_forward", "update"]
  ' <<< "${immutable_ruleset}" >/dev/null || {
    echo "core release tag immutability must cover every core release tag with no bypass" >&2
    exit 1
  }

if [[ "${scope}" == 'docs' ]]; then
  mapfile -t mutation_names < <(
    while IFS= read -r ruleset_id; do
      ruleset="$(gh api "/repos/${repository}/rulesets/${ruleset_id}?includes_parents=true")"
      if jq -e '
        any(.rules[]?;
          .type == "deletion" or
          .type == "update" or
          .type == "non_fast_forward"
        )
      ' <<< "${ruleset}" >/dev/null; then
        jq -r '.name' <<< "${ruleset}"
      fi
    done < <(jq -r '.[] | select(
      .target == "tag" and .enforcement == "active"
    ) | .id' <<< "${rulesets}")
  )
  mapfile -t mutation_names < <(printf '%s\n' "${mutation_names[@]}" | sort)
  if [[ "${#mutation_names[@]}" -ne 5 \
    || "${mutation_names[0]}" != 'docs-tags-controlled-deletion' \
    || "${mutation_names[1]}" != 'docs-tags-no-in-place-update' \
    || "${mutation_names[2]}" != 'release-tags-immutable' \
    || "${mutation_names[3]}" != 'v1.3.5-stopped-tags-never-create' \
    || "${mutation_names[4]}" != 'v1.3.6-stopped-tags-never-create' ]]; then
    echo 'exactly the core immutable, stopped-train freezes, and two Docs replacement rulesets may govern tag mutation' >&2
    exit 1
  fi

  docs_deletion_id="$(unique_ruleset_id docs-tags-controlled-deletion)"
  docs_deletion_ruleset="$(gh api "/repos/${repository}/rulesets/${docs_deletion_id}?includes_parents=true")"
  jq -e \
    --arg repository "${repository}" \
    --argjson release_actor_id "${release_actor_id}" '
    .source_type == "Repository" and
    .source == $repository and
    .target == "tag" and
    .enforcement == "active" and
    .conditions.ref_name.include == ["refs/tags/docs/v*"] and
    .conditions.ref_name.exclude == [] and
    [.rules[].type] == ["deletion"] and
    .bypass_actors == [{
      actor_id: $release_actor_id,
      actor_type: "User",
      bypass_mode: "always"
    }]
  ' <<< "${docs_deletion_ruleset}" >/dev/null || {
    echo 'Docs tag deletion authority must belong only to the explicit release actor' >&2
    exit 1
  }

  docs_no_update_id="$(unique_ruleset_id docs-tags-no-in-place-update)"
  docs_no_update_ruleset="$(gh api "/repos/${repository}/rulesets/${docs_no_update_id}?includes_parents=true")"
  jq -e \
    --arg repository "${repository}" '
    .source_type == "Repository" and
    .source == $repository and
    .target == "tag" and
    .enforcement == "active" and
    .conditions.ref_name.include == ["refs/tags/docs/v*"] and
    .conditions.ref_name.exclude == [] and
    ([.rules[].type] | sort) == ["non_fast_forward", "update"] and
    .bypass_actors == []
  ' <<< "${docs_no_update_ruleset}" >/dev/null || {
    echo 'Docs tags must reject in-place update and require delete then recreate' >&2
    exit 1
  }
fi

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

if [[ "${scope}" == 'core' ]]; then
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
else
  verify_active_environment prod \
    'docs/v*|tag'
fi

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

if [[ "${scope}" == 'core' ]]; then
  verify_environment_secrets release
  verify_environment_secrets release-v6
  verify_environment_secrets release-auto
  verify_environment_secrets release-v6-auto
  verify_environment_secrets npm-auto

  jq -n \
    --arg repository "${repository}" \
    --arg inspector "${inspector_login}" \
    --arg actor "${release_actor_login}" \
    --arg scope "${scope}" \
    --argjson controlled_ruleset_id "${controlled_id}" \
    --argjson stopped_v135_ruleset_id "${stopped_v135_id}" \
    --argjson stopped_v136_ruleset_id "${stopped_v136_id}" \
    --argjson immutable_ruleset_id "${immutable_id}" \
    '{
      success: true,
      scope: $scope,
      repository: $repository,
      inspector: $inspector,
      releaseActor: $actor,
      controlledCreationRuleset: $controlled_ruleset_id,
      stoppedV135CreationRuleset: $stopped_v135_ruleset_id,
      stoppedV136CreationRuleset: $stopped_v136_ruleset_id,
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
        npmAuto: ["refs/tags/v*"]
      },
      environmentSecrets: {
        release: [],
        releaseV6: [],
        releaseAuto: [],
        releaseV6Auto: [],
        npmAuto: []
      }
    }'
else
  verify_environment_secrets prod

  jq -n \
    --arg repository "${repository}" \
    --arg inspector "${inspector_login}" \
    --arg actor "${release_actor_login}" \
    --arg scope "${scope}" \
    --argjson controlled_ruleset_id "${controlled_id}" \
    --argjson stopped_v135_ruleset_id "${stopped_v135_id}" \
    --argjson stopped_v136_ruleset_id "${stopped_v136_id}" \
    --argjson core_immutable_ruleset_id "${immutable_id}" \
    --argjson docs_deletion_ruleset_id "${docs_deletion_id}" \
    --argjson docs_no_update_ruleset_id "${docs_no_update_id}" \
    '{
      success: true,
      scope: $scope,
      repository: $repository,
      inspector: $inspector,
      releaseActor: $actor,
      controlledCreationRuleset: $controlled_ruleset_id,
      stoppedV135CreationRuleset: $stopped_v135_ruleset_id,
      stoppedV136CreationRuleset: $stopped_v136_ruleset_id,
      coreImmutableRuleset: $core_immutable_ruleset_id,
      controlledDeletionRuleset: $docs_deletion_ruleset_id,
      noInPlaceUpdateRuleset: $docs_no_update_ruleset_id,
      tagMode: "delete-then-recreate",
      environments: {
        prod: ["refs/tags/docs/v*"]
      },
      environmentSecrets: {
        prod: []
      },
      docsCredential: {
        name: "CF_API_TOKEN",
        source: "organization",
        repositoryOverride: false,
        environmentOverride: false
      }
    }'
fi
