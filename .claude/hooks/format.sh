#!/usr/bin/env bash
# PostToolUse hook: formats the file Claude Code just edited.
# Claude Code passes the tool call as JSON on stdin (tool_input.file_path).
file=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))')

[ -f "$file" ] || exit 0
case "$file" in
  */vendor/*|*/ogen/*|*_templ.go) exit 0 ;;   # generated or third-party code
esac

cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
case "$file" in
  *.go)    mise exec -- gofmt -w "$file" ;;
  *.templ) mise exec -- templ fmt "$file" ;;
esac
exit 0
