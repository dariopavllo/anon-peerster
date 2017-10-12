@echo off
del gossiper.exe
del client.exe
cd part1
go build
cd ..
mv part1/part1.exe gossiper.exe
go build part1/client/client.go

rem start gossiper -name=Node1 -UIPort=4001 -gossipPort=5001 -peers=127.0.0.1:5002
rem start gossiper -name=Node2 -gossipPort=5002 -peers=127.0.0.1:5001,127.0.0.1:5003
rem start gossiper -name=Node3 -UIPort=4003 -gossipPort=5003 -peers=127.0.0.1:5002
rem start client -UIPort=4001 -msg=TestA
rem start client -UIPort=4003 -msg=TestB


start gossiper -name=NodeA -UIPort=4001 -gossipPort=127.0.0.1:5001 -peers=127.0.0.1:5005
start gossiper -name=NodeB -UIPort=4002 -gossipPort=127.0.0.1:5002 -peers=127.0.0.1:5006
start gossiper -name=NodeC -UIPort=4003 -gossipPort=127.0.0.1:5003 -peers=127.0.0.1:5007
start gossiper -name=NodeD -UIPort=4004 -gossipPort=127.0.0.1:5004 -peers=127.0.0.1:5008
start gossiper -name=Int5 -gossipPort=127.0.0.1:5005 -peers=127.0.0.1:5006
start gossiper -name=Int6 -gossipPort=127.0.0.1:5006 -peers=127.0.0.1:5007
start gossiper -name=Int7 -gossipPort=127.0.0.1:5007 -peers=127.0.0.1:5008
start gossiper -name=Int8 -gossipPort=127.0.0.1:5008

sleep 1
start client -UIPort=4001 -msg=TestA
sleep 1
start client -UIPort=4002 -msg=TestB
sleep 1
start client -UIPort=4003 -msg=TestC
sleep 1
start client -UIPort=4004 -msg=TestD