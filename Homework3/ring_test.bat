@echo off
del gossiper.exe
go build
mv Homework3.exe gossiper.exe

start "LeafA" cmd /K gossiper -name=LeafA -gossipAddr=127.0.0.1:5001 -peers=127.0.0.1:5005 -UIPort=8080
start "LeafB" cmd /K gossiper -name=LeafB -gossipAddr=127.0.0.1:5002 -peers=127.0.0.1:5006 -UIPort=8081
start "LeafC" cmd /K gossiper -name=LeafC -gossipAddr=127.0.0.1:5003 -peers=127.0.0.1:5007 -UIPort=8082
start "LeafD" cmd /K gossiper -name=LeafD -gossipAddr=127.0.0.1:5004 -peers=127.0.0.1:5008 -UIPort=8083
start "RingA" cmd /K gossiper -name=RingA -gossipAddr=127.0.0.1:5005 -peers=127.0.0.1:5006,127.0.0.1:5008,127.0.0.1:5001
start "RingB" cmd /K gossiper -name=RingB -gossipAddr=127.0.0.1:5006 -peers=127.0.0.1:5005,127.0.0.1:5007,127.0.0.1:5002
start "RingC" cmd /K gossiper -name=RingC -gossipAddr=127.0.0.1:5007 -peers=127.0.0.1:5006,127.0.0.1:5008,127.0.0.1:5003
start "RingD" cmd /K gossiper -name=RingD -gossipAddr=127.0.0.1:5008 -peers=127.0.0.1:5005,127.0.0.1:5007,127.0.0.1:5004