#!/bin/bash
DIR=$(pwd)
rm gossiper 2> /dev/null
echo "Compiling..."
go build
echo "Compiled."
mv Project gossiper

x-terminal-emulator -T LeafA -e $DIR/gossiper -dataDir=_data/LeafA -gossipAddr=127.0.0.1:5001 -peers=127.0.0.1:5005 -UIPort=8080
x-terminal-emulator -T LeafB -e $DIR/gossiper -dataDir=_data/LeafB -gossipAddr=127.0.0.1:5002 -peers=127.0.0.1:5006 -UIPort=8081
x-terminal-emulator -T LeafC -e $DIR/gossiper -dataDir=_data/LeafC -gossipAddr=127.0.0.1:5003 -peers=127.0.0.1:5007 -UIPort=8082
x-terminal-emulator -T LeafD -e $DIR/gossiper -dataDir=_data/LeafD -gossipAddr=127.0.0.1:5004 -peers=127.0.0.1:5008 -UIPort=8083
x-terminal-emulator -T RingA -e $DIR/gossiper -dataDir=_data/RingA -gossipAddr=127.0.0.1:5005 -peers=127.0.0.1:5006,127.0.0.1:5008,127.0.0.1:5001
x-terminal-emulator -T RingB -e $DIR/gossiper -dataDir=_data/RingB -gossipAddr=127.0.0.1:5006 -peers=127.0.0.1:5005,127.0.0.1:5007,127.0.0.1:5002
x-terminal-emulator -T RingC -e $DIR/gossiper -dataDir=_data/RingC -gossipAddr=127.0.0.1:5007 -peers=127.0.0.1:5006,127.0.0.1:5008,127.0.0.1:5003
x-terminal-emulator -T RingD -e $DIR/gossiper -dataDir=_data/RingD -gossipAddr=127.0.0.1:5008 -peers=127.0.0.1:5005,127.0.0.1:5007,127.0.0.1:5004
