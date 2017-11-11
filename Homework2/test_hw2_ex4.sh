#!/usr/bin/env bash
set -e

. ./test_lib.sh

crosscompile
restartdocker
startnodes nodea nodeb nodepub

log "Sending messages"
sendmsg nodea $message_c1_1
sendmsg nodeb $message_c2_1
sleep 3
sendpriv nodea nodeb $message_c2_2
sleep 3

log "Testing output"
test_grep nodea "CLIENT"
test_grep nodeb "CLIENT"
test_grep nodepub "MONGERING ROUTE with"
test_ngrep nodepub "MONGERING TEXT with"
test_grep nodea "DSDV nodeb"
test_grep nodeb "DSDV nodea"
test_ngrep nodeb "$message_c2_2"
test_grep nodepub "Not forwarding private message"
sleep 2

stopdocker
