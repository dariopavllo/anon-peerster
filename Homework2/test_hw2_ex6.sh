#!/usr/bin/env bash
set -e

. ./test_lib.sh

crosscompile
restartdocker
startnodes nodea nodeb nodepub
sleep 2

log "Sending messages"
sendmsg nodea $message_c1_1
sendmsg nodeb $message_c2_1
# sendmsg nodec $message_c3_1
sleep 10

grep DIRECT-ROUTE */gossip.log

log "Testing output"
test_grep nodeb "DIRECT-ROUTE FOR nodea: 172.16.0.3:10000"
test_grep nodea "DIRECT-ROUTE FOR nodeb: 172.16.0.4:10000"
# test_grep nodea "DIRECT-ROUTE FOR nodec: 172.16.0.4:1024"
