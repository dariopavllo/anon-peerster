#!/bin/bash
name=$(hostname)
ip=$( nslookup $name | grep Address | cut -d " " -f 3)
/root/gossiper -UIPort=10001 -gossipAddr=$ip:10000 -name=$name \
  -peers=172.16.0.2:10000 -rtimer=1 $@ 2>&1 > /root/gossip.log
