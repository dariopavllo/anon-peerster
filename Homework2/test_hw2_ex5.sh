#!/usr/bin/env bash
set -e

. ./test_lib.sh

crosscompile
restartdocker
startnodes nodea nodeb nodec nodepub
sleep 3

log "Sending messages"
sendmsg nodea $message_c1_1
sendmsg nodeb $message_c2_1
sendmsg nodec $message_c3_1
sleep 15

log "Testing output"
test_grep nodepub "DSDV nodea: 172.16.0.3:10000"
test_grep nodepub "DSDV nodeb: 172.16.0.4:10000"
test_grep nodepub "DSDV nodec: 172.16.0.4:10"
test_ngrep nodepub "DSDV nodec: 172.16.0.4:10000"

stopdocker
