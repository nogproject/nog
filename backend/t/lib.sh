sharness=./sharness/sharness.sh

isContainer() {
    [ -d '/go' ]
}

if ! isContainer; then
    echo >&2 'fatal: The tests must be executed in the godev container.'
    exit 1
fi

source "${sharness}"

# Nanoseconds since UNIX epoch.
nanos() {
    date +%s%N
}

# Example: `waitPort 5s 7550`.
waitPort() {
    local timeout="$1"
    timeout=$(tr -d 's' <<< "${timeout}")
    local port="$2"
    local w=0.1
    local n=$(( $timeout * 10 ))
    while ! nc -z localhost ${port}; do
        if ! let n--; then
            return 1
        fi
        sleep ${w}
    done
}

# Example: `waitGrep 5s 'Initialized shadow' stad.log`.
waitGrep() {
    local timeout="$1"
    timeout=$(tr -d 's' <<< "${timeout}")
    shift
    local w=0.1
    local n=$(( $timeout * 10 ))
    while ! grep -q "$@"; do
        if ! let n--; then
            return 1
        fi
        sleep ${w}
    done
}

# Example: `logRotate stad.log`.
logRotate() {
    local log="$1"
    cat "${log}" >"${log}_$(nanos)"
    printf '' >"${log}"
}

# Use a separate database for each test run.  Names are based on Unix nanos.
# Usage:
#
# ```
# trap cleanupFsod EXIT
# test_expect_success 'start fsods' 'startFsods'
# ...
# test_expect_success 'shutdown fsods' 'shutdownFsods'
# test_done
# ```
#
# Available after startFsods:
#
#  - Global variable `fsoExampleNs`.
#  - FSO root `/${fsoExampleNs}/d a t` and testing dir `d a t`.
#

# Allow forwarding wildcard tokens in `../internal/nogfsoregd/statdsd/auth.go`.
export TESTING_INSECURE_NOGFSOREGD_FORWARD_TOKEN=1

startFsods() {
    startFsodsOpts '' ''
}

startFsodsStadOpts() {
    startFsodsOpts '' "$*"
}

startFsodsOpts() {
    local regdOpts="$1"
    local stadOpts="$2"

    local ts=$(nanos)
    local mongoUrl="%2Fmongo%2Frun%2Fmongodb-27017.sock/nogfsoreg-t${ts}"
    fsoExampleNs="example-t${ts}"

    nogfsoregd ${regdOpts} --log=mu --mongodb="${mongoUrl}" 2>&1 \
    | tee regd.log &
    if ! waitPort 5s 7550; then
        return 1
    fi
    if ! waitPort 5s 7551; then
        return 1
    fi

    if ! nogfsoctl init registry --no-vid exreg; then
        return 1
    fi

    if ! mkdir shadow 'd a t'; then
        return 1
    fi

    nogfsostad ${stadOpts} --log=mu \
        --shadow-root="$PWD/shadow" \
        --host=files.example.com \
        --prefix="/${fsoExampleNs}/d a t" \
        --gitlab-token=/etc/gitlab/root.token \
        exreg 2>&1 | tee stad.log &
    if ! waitGrep 5s 'GRPC listening disabled' stad.log; then
        return 1
    fi

    if ! nogfsoctl init root exreg --no-vid \
        --host=files.example.com \
        --gitlab-namespace=localhost/root \
        "/${fsoExampleNs}/d a t" \
        "$PWD/d a t";
    then
        return 1
    fi
}

# Graceful.
shutdownFsods() {
    kill -s TERM $(pgrep nogfsostad) $(pgrep nogfsoregd)
    if ! waitGrep 5s "Completed graceful shutdown" stad.log; then
        return 1
    fi
    if ! waitGrep 5s "Completed graceful shutdown" regd.log; then
        return 1
    fi
}

# Forced.
cleanupFsods() {
    (
        pgrep nogfsoregd || true
        pgrep nogfsostad || true
    ) \
    | xargs --verbose --no-run-if-empty kill -s KILL
}

# `getRepoId <registry> <repoName>` prints the `<repoUUID>`.
getRepoId() {
    local reg="$1"
    local name="$2"
    nogfsoctl get repos "${reg}" \
    | grep -- "^- .*${name}" \
    | cut -b 3- \
    | jq -r .id \
    | head -n 1 \
    | egrep '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
}
