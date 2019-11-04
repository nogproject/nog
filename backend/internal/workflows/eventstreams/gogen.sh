#!/bin/bash
set -o errexit -o nounset -o pipefail -o noglob

dir='backend/internal/workflows/eventstreams'

out='registry-workflow-index.go'
echo "    GEN ${dir}/${out}"
sed \
  -e 's,//go:generate.*,// Created by go generate; DO NOT EDIT.,' \
  -e 's/RegistryWorkflowEvents/RegistryWorkflowIndexEvents/g' \
  <'registry-workflow.go' >"${out}"
