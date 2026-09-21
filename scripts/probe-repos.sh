#!/usr/bin/env bash
set -euo pipefail

out="${1:-$HOME/.gradle/repo-order.properties}"

ids=(google mavenCentral tencentGoogle tencentCentral huawei tuna cernet jitpack)

declare -A roots=(
  [google]=https://dl.google.com
  [mavenCentral]=https://repo1.maven.org
  [tencentGoogle]=https://mirrors.tencent.com/nexus/repository/google/
  [tencentCentral]=https://mirrors.tencent.com/nexus/repository/maven-central/
  [huawei]=https://repo.huaweicloud.com/repository/maven/
  [tuna]=https://mirrors.tuna.tsinghua.edu.cn/maven/
  [cernet]=https://mirrors.cernet.edu.cn/maven/
  [jitpack]=https://jitpack.io
)

declare -A specs=(
  [google]=google
  [mavenCentral]=mavenCentral
  [tencentGoogle]='https://mirrors.tencent.com/nexus/repository/google/'
  [tencentCentral]='https://mirrors.tencent.com/nexus/repository/maven-central/'
  [huawei]='https://repo.huaweicloud.com/repository/maven/'
  [tuna]='https://mirrors.tuna.tsinghua.edu.cn/maven/'
  [cernet]='https://mirrors.cernet.edu.cn/maven/'
  [jitpack]='https://jitpack.io'
)

scored=()
for id in "${ids[@]}"; do
  latency=$(curl -s -o /dev/null -m 6 --connect-timeout 4 -w '%{time_connect}' "${roots[$id]}" 2>/dev/null || true)
  case "$latency" in
    ''|*0.000000*) echo "unreachable: $id"; continue ;;
  esac
  printf 'probe %-15s %ss\n' "$id" "$latency"
  scored+=("$latency:$id")
done

ordered=""
if [ "${#scored[@]}" -gt 0 ]; then
  while IFS= read -r id; do
    [ "$id" = cernet ] && continue
    ordered="$ordered$id,"
  done < <(printf '%s\n' "${scored[@]}" | sort -n -t: -k1,1 | cut -d: -f2)
fi

for id in "${ids[@]}"; do
  [ "$id" = cernet ] && continue
  case ",$ordered" in
    *",$id,"*) ;;
    *) ordered="$ordered$id," ;;
  esac
done
ordered="${ordered%,},cernet"

list=""
IFS=',' read -ra sequence <<< "$ordered"
for id in "${sequence[@]}"; do
  list="$list${specs[$id]},"
done
list="${list%,}"

mkdir -p "$(dirname "$out")"
printf 'order=%s\n' "$list" > "$out"
echo "repository order: $list"
