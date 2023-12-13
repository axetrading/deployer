#!/bin/sh

set -e

wait_for_file_to_exist() {
  local filename; filename="$1"
  while true; do
    if [ -f "$filename" ]; then
      break
    fi
    sleep 1
  done
}

rm -rf /control/commands
rm -rf /control/output
rm -f /control/run.sh

mkdir /control/commands
mkdir /control/output

cp run.sh /control/run.sh

echo "terraform -version" > /control/commands/version
wait_for_file_to_exist /control/output/version
cat /control/output/version

wait_for_file_to_exist /control/output/version.status
echo "version status: $(cat /control/output/version.status)"

touch /control/commands/done