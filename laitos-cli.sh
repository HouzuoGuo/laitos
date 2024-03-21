#!/usr/bin/env bash

# A WIP laitos cli shell script.

set -Eeuo pipefail
export LANG=C LC_ALL=C

handle_exit() {
    local -r -i exit_status="$?"
    echo "exiting with status code $exit_status"
    exit $exit_status
}

trap handle_exit INT HUP TERM QUIT EXIT

print_usage() {
    local -r -i exit_status="$1"
    echo "$0 - laitos client CLI"
    echo "-u, --url"
    printf "\tInfo endpoint URLs (e.g. https://example.com/laitos-info), separated by comma, may be repeated.\n"
    exit $exit_status
}

main() {
    local -a url_argv=() urls=()
    while true; do
        local arg_val=''
        if [ $# -eq 0 ]; then
            break
        fi
        case "$1" in
            -h | --help)
                print_usage 0
                ;;
            --url=*)
                arg_val="${1#*=}"
                ;&
            -u | --url)
                if [ ! "$arg_val" ]; then
                    shift
                    [ $# -ge 1 ] && arg_val="$1"
                fi
                [ ! "$arg_val" ] && print_usage 1
                url_argv+=("$arg_val")
                ;;
        esac
        shift
    done
    # Split possibly comma separated server arg values.
    for argv in "${url_argv[@]}"; do
        while IFS=',' read -r -a fields; do
            urls+=("${fields[@]}")
        done <<<"$argv"
    done

    printf "Fetching server info from %s URLs...\n" "${#urls[@]}"

    local -A resp=()
    for url in "${urls[@]}"; do
        mapfile -d $'\0' resp_body < <(curl -H 'Accept: application/json' "$url" 2>/dev/null)
        resp["$url"]="$resp_body"
        wait
    done

    for key in "${!resp[@]}"; do
        local val="${resp["$key"]}"
        printf "%s response length %s\n" "$key" "${#val}"
    done
}

main "${@}"
