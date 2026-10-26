#!/bin/sh
# Stands in for mdreader in the plugin tests: records its arguments and
# everything it receives to $MDR_FAKE_LOG. Behaviour knobs:
#   MDR_FAKE_OPEN=<url>   print "OPEN <url>" on start
#   MDR_FAKE_FAIL=1       complain on stderr and exit 3 right away
#   MDR_FAKE_STUBBORN=1   after stdin EOF, ignore SIGTERM and keep running
if [ "$MDR_FAKE_FAIL" = 1 ]; then
  echo "fake viewer failed" >&2
  exit 3
fi
echo "ARGS $*" > "$MDR_FAKE_LOG"
[ -n "$MDR_FAKE_OPEN" ] && echo "OPEN $MDR_FAKE_OPEN"
cat >> "$MDR_FAKE_LOG"
echo "EOF" >> "$MDR_FAKE_LOG"
if [ "$MDR_FAKE_STUBBORN" = 1 ]; then
  trap '' TERM
  exec sleep 30
fi
