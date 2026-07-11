#!/bin/bash
#
# node  --  DEMO "malware" masquerading as the node interpreter.
#
# This is a HARMLESS teaching prop for a container-isolation talk. It pretends
# to be the node binary, but instead of running JS it *probes* every isolation
# boundary a container is supposed to give you and reports whether that boundary
# would actually hold. It is strictly READ-ONLY: it never forks a bomb, never
# kills a process, never mounts, unmounts, or writes outside its own log file.
# Each check just inspects /proc and /sys to decide "could I have escaped here?"
#
# Workflow for the talk: run `node` with NO namespaces -> lots of ESCAPED.
# Add one CLONE_NEW* flag at a time to your runtime, re-run, and watch the
# matching row flip to CONTAINED. The real interpreter is preserved as
# `node.real`; the probe hands off to it automatically when given something to
# run (e.g. `node app.js`), or when EVILNODE_REAL=1 -- so the backdoored app
# still starts after the probe has quietly logged its findings.
#
set -u

# ----------------------------------------------------------------------------
# setup: log destination + colour
# ----------------------------------------------------------------------------
LOG="${EVILNODE_LOG:-/var/log/evilnode.log}"
if ! { : >>"$LOG"; } 2>/dev/null; then
    LOG="/tmp/evilnode.log"
    : >>"$LOG" 2>/dev/null || LOG="/dev/null"
fi

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    C_RED=$'\033[31m'
    C_GRN=$'\033[32m'
    C_YEL=$'\033[33m'
    C_CYN=$'\033[36m'
    C_DIM=$'\033[2m'
    C_BLD=$'\033[1m'
    C_RST=$'\033[0m'
else
    C_RED=
    C_GRN=
    C_YEL=
    C_CYN=
    C_DIM=
    C_BLD=
    C_RST=
fi

# no Date.now() gymnastics; date(1) is fine inside the container
STAMP="$(date '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo unknown)"

# summary rows collected as "Namespace|Flag|STATUS"
SUMMARY=()

# ----------------------------------------------------------------------------
# helpers
# ----------------------------------------------------------------------------
# plain line -> logfile only
logf() { printf '%s\n' "$*" >>"$LOG" 2>/dev/null; }

# section header -> screen + log
section() {
    printf '\n%s%s== %s ==%s\n' "$C_BLD" "$C_CYN" "$1" "$C_RST"
    logf ""
    logf "== $1 =="
}

# evidence line (dim on screen)
note() {
    printf '   %s%s%s\n' "$C_DIM" "$1" "$C_RST"
    logf "   $1"
}

# verdict:  <status> <human sentence>   where status is ESCAPED|CONTAINED|LEAK
verdict() {
    local status="$1"
    shift
    local msg="$*" colour tag
    case "$status" in
    ESCAPED)
        colour="$C_RED"
        tag="[ESCAPED]  "
        ;;
    CONTAINED)
        colour="$C_GRN"
        tag="[CONTAINED]"
        ;;
    LEAK)
        colour="$C_YEL"
        tag="[LEAK]     "
        ;;
    *)
        colour="$C_YEL"
        tag="[INFO]     "
        ;;
    esac
    printf '   %s%s%s %s\n' "$colour" "$tag" "$C_RST" "$msg"
    logf "   $tag $msg"
}

record() { SUMMARY+=("$1|$2|$3"); }

# read first line of a file, empty string if missing/unreadable
readfirst() { head -n1 "$1" 2>/dev/null || printf ''; }

# ----------------------------------------------------------------------------
# banner
# ----------------------------------------------------------------------------
printf '%s%s' "$C_RED" "$C_BLD"
cat <<'BANNER'
  ____ _  _ _ _    _  _ ____ ___  ____
  |___ |  | | |    |\ | |  | |  \ |___
  |___  \/  | |___ | \| |__| |__/ |___   pretending to be `node`
BANNER
printf '%s' "$C_RST"
printf '%s  read-only container-escape probe  --  log: %s%s\n' "$C_DIM" "$LOG" "$C_RST"

logf "############################################################"
logf "# evilnode escape probe @ $STAMP"
logf "# uid=$(id -u 2>/dev/null) gid=$(id -g 2>/dev/null) host=$(cat /proc/sys/kernel/hostname 2>/dev/null)"
logf "############################################################"

# ============================================================================
# 0. FORK BOMB  (cgroup pids controller)  -- reported, never detonated
# ============================================================================
section "Fork bomb  (cgroup: pids controller)"
pidmax=""
if [ -r /sys/fs/cgroup/pids.max ]; then
    pidmax="$(readfirst /sys/fs/cgroup/pids.max)" # cgroup v2
    pidcur="$(readfirst /sys/fs/cgroup/pids.current)"
elif [ -r /sys/fs/cgroup/pids/pids.max ]; then
    pidmax="$(readfirst /sys/fs/cgroup/pids/pids.max)" # cgroup v1
    pidcur="$(readfirst /sys/fs/cgroup/pids/pids.current)"
fi
ulim="$(ulimit -u 2>/dev/null)"
note "cgroup pids.max = ${pidmax:-<none>}   pids.current = ${pidcur:-?}   ulimit -u = ${ulim:-?}"
note "(the classic bomb :(){ :|:& };:  is NOT being run -- only measured)"
if [ -z "$pidmax" ] || [ "$pidmax" = "max" ]; then
    if [ "$ulim" = "unlimited" ]; then
        verdict ESCAPED "No pids cap. A fork bomb would exhaust host PIDs and wedge the box."
        record "Fork bomb" "cgroup pids" "ESCAPED"
    else
        verdict LEAK "No cgroup pids cap; only RLIMIT_NPROC=$ulim stands between you and the host."
        record "Fork bomb" "cgroup pids" "LEAK"
    fi
else
    verdict CONTAINED "Fork bomb capped at $pidmax processes by the cgroup -- host survives."
    record "Fork bomb" "cgroup pids" "CONTAINED"
fi

# ============================================================================
# 1. PID  (CLONE_NEWPID)
# ============================================================================
section "PID namespace  (CLONE_NEWPID)"
nproc_seen=0
for p in /proc/[0-9]*; do nproc_seen=$((nproc_seen + 1)); done
pid1="$(readfirst /proc/1/comm)"
note "visible processes = $nproc_seen    PID 1 = '${pid1:-?}'"
case "$pid1" in
systemd | init | "")
    verdict ESCAPED "PID 1 is the host init and $nproc_seen procs are visible -- host process table exposed."
    record "PID" "CLONE_NEWPID" "ESCAPED"
    ;;
*)
    if [ "$nproc_seen" -gt 50 ]; then
        verdict ESCAPED "$nproc_seen processes visible -- looks like the host's process table."
        record "PID" "CLONE_NEWPID" "ESCAPED"
    else
        verdict CONTAINED "Own PID namespace: PID 1 = '$pid1', only $nproc_seen procs visible."
        record "PID" "CLONE_NEWPID" "CONTAINED"
    fi
    ;;
esac

# ============================================================================
# 2. MOUNT  (CLONE_NEWNS)
# ============================================================================
section "Mount namespace  (CLONE_NEWNS)"
nmounts="$(wc -l </proc/mounts 2>/dev/null | tr -d ' ')"
has_overlay="$(grep -c 'overlay / ' /proc/mounts 2>/dev/null)"
[ -e /.dockerenv ] && dockerenv="present" || dockerenv="absent"
# host give-aways: real block devices or the host's /etc mounted through
host_dev="$(grep -Ec ' /(host|etc/hostname|etc/resolv.conf) ' /proc/mounts 2>/dev/null)"
note "mount entries = ${nmounts:-?}   overlay-root = ${has_overlay}   /.dockerenv = ${dockerenv}"
if grep -qE '^/dev/(sd|nvme|vd)' /proc/mounts 2>/dev/null; then
    verdict ESCAPED "Raw host block devices ($(grep -oE '^/dev/(sd|nvme|vd)[a-z0-9]*' /proc/mounts | head -1)) are mounted -- host disk reachable."
    record "Mount" "CLONE_NEWNS" "ESCAPED"
elif [ "${nmounts:-0}" -gt 40 ]; then
    verdict LEAK "$nmounts mount entries -- unusually many; host mount table may be leaking in."
    record "Mount" "CLONE_NEWNS" "LEAK"
else
    verdict CONTAINED "Private mount table ($nmounts entries) over an isolated rootfs."
    record "Mount" "CLONE_NEWNS" "CONTAINED"
fi

# ============================================================================
# 3. NETWORK  (CLONE_NEWNET)
# ============================================================================
section "Network namespace  (CLONE_NEWNET)"
ifaces=""
for n in /sys/class/net/*; do [ -e "$n" ] && ifaces="$ifaces ${n##*/}"; done
ifaces="${ifaces# }"
note "interfaces =${ifaces:+ }${ifaces:-<none>}"
# Host physical NICs and bridges that only appear when you share the host's
# network stack. NOTE: a lone `eth0` is NOT a tell -- it's the veth a fresh
# netns gets, so matching it would flag the CONTAINED case as an escape.
hostnic_re='^(wlp|wlan|enp|eno|ens|enx|docker0|virbr|incusbr|br-)'
if printf '%s\n' $ifaces | grep -qE "$hostnic_re"; then
    seen="$(printf '%s\n' $ifaces | grep -oE "${hostnic_re#^}[a-z0-9-]*" | tr '\n' ' ')"
    verdict ESCAPED "Host NIC(s)/bridge(s) visible (${seen})-- sharing the host network stack."
    record "Network" "CLONE_NEWNET" "ESCAPED"
else
    # only lo, or lo + a lone veth (typically eth0) == our own netns
    verdict CONTAINED "Own network namespace: interfaces [${ifaces:-none}], no host NICs or bridges exposed."
    record "Network" "CLONE_NEWNET" "CONTAINED"
fi

# ============================================================================
# 4. UTS  (CLONE_NEWUTS)
# ============================================================================
section "UTS namespace  (CLONE_NEWUTS)"
hn="$(cat /proc/sys/kernel/hostname 2>/dev/null)"
# CAP_SYS_ADMIN bit (0x0000000000200000) in effective set => could sethostname()
capeff="$(grep -i '^CapEff:' /proc/self/status 2>/dev/null | awk '{print $2}')"
has_sysadmin=0
if [ -n "$capeff" ]; then
    # bit 21 = CAP_SYS_ADMIN
    if [ $((0x$capeff >> 21 & 1)) -eq 1 ] 2>/dev/null; then has_sysadmin=1; fi
fi
note "hostname = '${hn:-?}'    CapEff = ${capeff:-?}    CAP_SYS_ADMIN = $has_sysadmin"
# a container with its own UTS ns usually gets a random 12-hex-char hostname
if printf '%s' "$hn" | grep -qE '^[0-9a-f]{12}$'; then
    verdict CONTAINED "Container-style hostname '$hn' -- own UTS namespace; hostname changes stay local."
    record "UTS" "CLONE_NEWUTS" "CONTAINED"
elif [ "$has_sysadmin" -eq 1 ]; then
    verdict ESCAPED "Hostname '$hn' + CAP_SYS_ADMIN and no private UTS ns -- sethostname() would rename the HOST."
    record "UTS" "CLONE_NEWUTS" "ESCAPED"
else
    verdict LEAK "Hostname '$hn' looks host-shared, but no CAP_SYS_ADMIN to change it."
    record "UTS" "CLONE_NEWUTS" "LEAK"
fi

# ============================================================================
# 5. IPC  (CLONE_NEWIPC)
# ============================================================================
section "IPC namespace  (CLONE_NEWIPC)"
# header line is always present, so subtract 1 for object count
shm=$(($(wc -l </proc/sysvipc/shm 2>/dev/null || echo 1) - 1))
sem=$(($(wc -l </proc/sysvipc/sem 2>/dev/null || echo 1) - 1))
msg=$(($(wc -l </proc/sysvipc/msg 2>/dev/null || echo 1) - 1))
[ "$shm" -lt 0 ] && shm=0
[ "$sem" -lt 0 ] && sem=0
[ "$msg" -lt 0 ] && msg=0
note "SysV objects -> shm=$shm  sem=$sem  msg=$msg"
if [ $((shm + sem + msg)) -gt 0 ]; then
    verdict ESCAPED "$((shm + sem + msg)) host SysV IPC object(s) visible -- shared IPC namespace, can read/poke host shared memory."
    record "IPC" "CLONE_NEWIPC" "ESCAPED"
else
    verdict CONTAINED "No SysV IPC objects visible -- own (empty) IPC namespace."
    record "IPC" "CLONE_NEWIPC" "CONTAINED"
fi

# ============================================================================
# 6. USER  (CLONE_NEWUSER)
# ============================================================================
section "User namespace  (CLONE_NEWUSER)"
uid="$(id -u 2>/dev/null)"
euid="${EUID:-$uid}"
uidmap="$(tr -s ' ' </proc/self/uid_map 2>/dev/null | sed 's/^ //')"
note "uid = ${uid:-?}    /proc/self/uid_map = '${uidmap:-<none>}'"
# "0 0 4294967295" => in-container root IS real host root (no userns)
if [ -z "$uidmap" ] || printf '%s' "$uidmap" | grep -qE '^0 0 '; then
    if [ "${uid:-1}" = "0" ]; then
        verdict ESCAPED "uid 0 maps straight to host uid 0 -- you ARE real root; any host-root exploit lands."
        record "User" "CLONE_NEWUSER" "ESCAPED"
    else
        verdict LEAK "No user namespace, running as uid $uid -- unprivileged but sharing host UID space."
        record "User" "CLONE_NEWUSER" "LEAK"
    fi
else
    hostbase="$(printf '%s' "$uidmap" | awk '{print $2}')"
    verdict CONTAINED "User namespace active: container uid 0 -> host uid $hostbase (unprivileged outside)."
    record "User" "CLONE_NEWUSER" "CONTAINED"
fi

# ============================================================================
# 7. CGROUP  (CLONE_NEWCGROUP)
# ============================================================================
section "Cgroup namespace  (CLONE_NEWCGROUP)"
cg="$(readfirst /proc/self/cgroup)"
note "/proc/self/cgroup = '${cg:-?}'"
# with a cgroup ns you see a namespaced root (0::/ or a short path); without,
# the full host hierarchy leaks (system.slice, user.slice, docker-<id>.scope...)
if printf '%s' "$cg" | grep -qE '(system\.slice|user\.slice|machine\.slice|docker-|\.scope|/user@)'; then
    verdict LEAK "cgroup path leaks host layout ('${cg#*::}') -- no cgroup namespace; reveals host slice/scope names."
    record "Cgroup" "CLONE_NEWCGROUP" "LEAK"
elif [ "$cg" = "0::/" ] || printf '%s' "$cg" | grep -qE '::/$'; then
    verdict CONTAINED "cgroup root is namespaced ('$cg') -- host hierarchy hidden."
    record "Cgroup" "CLONE_NEWCGROUP" "CONTAINED"
else
    verdict CONTAINED "cgroup path is a short namespaced root ('$cg')."
    record "Cgroup" "CLONE_NEWCGROUP" "CONTAINED"
fi

# ============================================================================
# 8. TIME  (CLONE_NEWTIME)
# ============================================================================
section "Time namespace  (CLONE_NEWTIME)"
uptime_s="$(awk '{print int($1)}' /proc/uptime 2>/dev/null)"
if [ -e /proc/self/timens_offsets ]; then
    offs="$(tr '\n' ';' </proc/self/timens_offsets 2>/dev/null)"
    note "uptime = ${uptime_s:-?}s    timens_offsets = '${offs%;}'"
    # any non-zero offset column => a time namespace is actively shifting the clock
    if awk 'NF>=3 && ($2!=0 || $3!=0){f=1} END{exit !f}' /proc/self/timens_offsets 2>/dev/null; then
        verdict CONTAINED "Time namespace active with non-zero offsets -- boot/monotonic clocks are virtualised."
        record "Time" "CLONE_NEWTIME" "CONTAINED"
    else
        verdict LEAK "timens_offsets present but all zero -- clock still tracks host uptime (${uptime_s}s)."
        record "Time" "CLONE_NEWTIME" "LEAK"
    fi
else
    note "uptime = ${uptime_s:-?}s    timens_offsets = <absent>"
    verdict LEAK "No time namespace: container reads the host boot clock directly (uptime ${uptime_s}s)."
    record "Time" "CLONE_NEWTIME" "LEAK"
fi

# ============================================================================
# summary table
# ============================================================================
section "Summary"
printf '   %s%-11s %-16s %s%s\n' "$C_BLD" "Namespace" "Flag" "Status" "$C_RST"
printf '   %s%s%s\n' "$C_DIM" "----------- ---------------- ----------" "$C_RST"
esc=0
for row in "${SUMMARY[@]}"; do
    IFS='|' read -r ns flag st <<<"$row"
    case "$st" in
    ESCAPED)
        col="$C_RED"
        esc=$((esc + 1))
        ;;
    CONTAINED) col="$C_GRN" ;;
    *)
        col="$C_YEL"
        esc=$((esc + 1))
        ;;
    esac
    printf '   %-11s %-16s %s%s%s\n' "$ns" "$flag" "$col" "$st" "$C_RST"
    logf "   $ns | $flag | $st"
done
printf '\n   %s%d of %d boundaries would let me ESCAPE or LEAK.%s\n' \
    "$C_BLD" "$esc" "${#SUMMARY[@]}" "$C_RST"
logf ""
logf "SUMMARY: $esc/${#SUMMARY[@]} boundaries escapable @ $STAMP"

printf '%s   full log appended to %s%s\n\n' "$C_DIM" "$LOG" "$C_RST"

# ----------------------------------------------------------------------------
# hand off to the real interpreter
# ----------------------------------------------------------------------------
# A convincing backdoor still has to run the app. So after probing: if we were
# handed something to execute (e.g. `node /app/server.js`), or EVILNODE_REAL=1
# is set, exec the genuine interpreter with the same args. Bare `node` with no
# args just probes and exits -- that's the on-stage escape demo.
REAL="$(dirname "$0")/node.real"
if [ -x "$REAL" ] && { [ "$#" -gt 0 ] || [ "${EVILNODE_REAL:-0}" = "1" ]; }; then
  exec "$REAL" "$@"
fi
exit 0
