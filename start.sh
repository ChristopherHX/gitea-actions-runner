#!/usr/bin/env bash

if [[ ! -d /data ]]; then
  mkdir -p /data
fi

cd /data

RUNNER_STATE_FILE=${RUNNER_STATE_FILE:-'.runner'}

EXTRA_ARGS=""
if [[ ! -z "${GITEA_RUNNER_LABELS}" ]]; then
  EXTRA_ARGS="${EXTRA_ARGS} --labels ${GITEA_RUNNER_LABELS}"
fi
if [[ ! -z "${GITEA_RUNNER_EPHEMERAL}" ]]; then
  EXTRA_ARGS="${EXTRA_ARGS} --ephemeral"
fi

# In case no token is set, it's possible to read the token from a file, i.e. a Docker Secret
if [[ -z "${GITEA_RUNNER_REGISTRATION_TOKEN}" ]] && [[ -f "${GITEA_RUNNER_REGISTRATION_TOKEN_FILE}" ]]; then
  GITEA_RUNNER_REGISTRATION_TOKEN=$(cat "${GITEA_RUNNER_REGISTRATION_TOKEN_FILE}")
fi

# Use the same ENV variable names as https://github.com/vegardit/docker-gitea-act-runner
test -f "$RUNNER_STATE_FILE" || echo "$RUNNER_STATE_FILE is missing or not a regular file"

if [[ ! -s "$RUNNER_STATE_FILE" ]]; then
  try=1
  success=0

  # The point of this loop is to make it simple, when running both act_runner and gitea in docker,
  # for the act_runner to wait a moment for gitea to become available before erroring out.  Within
  # the context of a single docker-compose, something similar could be done via healthchecks, but
  # this is more flexible.
  while [[ $success -eq 0 ]] && [[ $try -lt ${GITEA_MAX_REG_ATTEMPTS:-10} ]]; do
    /runner/gitea-actions-runner register \
      --instance "${GITEA_INSTANCE_URL}" \
      --token    "${GITEA_RUNNER_REGISTRATION_TOKEN}" \
      --name     "${GITEA_RUNNER_NAME:-`hostname`}" \
      --worker python3,/runner/actions-runner-worker.py,/home/runner/bin/Runner.Worker \
      ${EXTRA_ARGS} --no-interactive 2>&1 | tee /tmp/reg.log

    cat /tmp/reg.log | grep 'Runner registered successfully' > /dev/null
    if [[ $? -eq 0 ]]; then
      echo "SUCCESS"
      success=1

      jq --null-input \
         --arg agentName "${GITEA_RUNNER_NAME:-`hostname`}" \
         --arg workFolder "/home/runner/_work" \
        '{ "isHostedServer": false, "agentName": $agentName, "workFolder": $workFolder }' > /data/.actions_runner
    else
      echo "Waiting to retry ..."
      try=$(($try + 1))
      sleep 5
    fi
  done
fi

if [[ ! -s "/data/.actions_runner" ]]; then
  jq --null-input \
         --arg agentName "${GITEA_RUNNER_NAME:-`hostname`}" \
         --arg workFolder "/home/runner/_work" \
        '{ "isHostedServer": false, "agentName": $agentName, "workFolder": $workFolder }' > /data/.actions_runner
fi

# Prevent reading the token from the act_runner process
unset GITEA_RUNNER_REGISTRATION_TOKEN
unset GITEA_RUNNER_REGISTRATION_TOKEN_FILE

/runner/gitea-actions-runner daemon "$@" ${GITEA_RUNNER_ONCE+"--once"}
