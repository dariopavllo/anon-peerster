@echo off
del gossiper.exe
go build
mv Homework2.exe gossiper.exe

rem start gossiper -name=NodeA -gossipAddr=127.0.0.1:5001 -peers=127.0.0.1:5002 -UIPort=8080
rem start gossiper -name=NodeB -gossipAddr=127.0.0.1:5002 -peers=127.0.0.1:5001,127.0.0.1:5003
rem start gossiper -name=NodeC -gossipAddr=127.0.0.1:5003 -peers=127.0.0.1:5002 -UIPort=8081

start gossiper -name=NodeA -gossipAddr=127.0.0.1:5001 -peers=127.0.0.1:5005 -UIPort=8080
start gossiper -name=NodeB -gossipAddr=127.0.0.1:5002 -peers=127.0.0.1:5006
start gossiper -name=NodeC -gossipAddr=127.0.0.1:5003 -peers=127.0.0.1:5007 -UIPort=8081
start gossiper -name=NodeD -gossipAddr=127.0.0.1:5004 -peers=127.0.0.1:5008 -disableTraversal
start gossiper -name=Int5 -gossipAddr=127.0.0.1:5005 -peers=127.0.0.1:5006,127.0.0.1:5008,127.0.0.1:5001 -noforward
start gossiper -name=Int6 -gossipAddr=127.0.0.1:5006 -peers=127.0.0.1:5005,127.0.0.1:5007,127.0.0.1:5002 -noforward
start gossiper -name=Int7 -gossipAddr=127.0.0.1:5007 -peers=127.0.0.1:5006,127.0.0.1:5008,127.0.0.1:5003 -noforward
start gossiper -name=Int8 -gossipAddr=127.0.0.1:5008 -peers=127.0.0.1:5005,127.0.0.1:5007,127.0.0.1:5004 -noforward